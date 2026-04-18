import { Router } from 'express';

/**
 * Creates the AT menu routes as an Express router.
 * Menu versions are append-only documents in Cosmos DB (HomepageDB/userdata).
 * Each save creates a new version. GET returns the latest. No mutations.
 *
 * Menu trees may contain `{ ref: "bookmarks" }` nodes that resolve at read
 * time to the caller's homepage bookmarks (from bookmarksContainer). On
 * write, the resolved form is stripped back to the pointer — edits under
 * the bookmark subtree are read-only from automate's side and aren't
 * written through to the bookmarks doc.
 *
 * @param {{
 *   requireAuth: Function,
 *   container: import('@azure/cosmos').Container,
 *   bookmarksContainer?: import('@azure/cosmos').Container,
 * }} opts
 */
export function createATRoutes({ requireAuth, container, bookmarksContainer }) {
  const router = Router();

  function versionDocId(userId, version) {
    return `menu_${userId}_v${version}`;
  }

  // Read the caller's latest bookmarks doc from the fzt-frontend-data
  // container. Returns { bookmarks, version, updatedAt } or null.
  async function getLatestBookmarks(userId) {
    if (!bookmarksContainer) return null;
    const { resources } = await bookmarksContainer.items.query({
      query: `SELECT TOP 1 * FROM c
              WHERE c.type = 'bookmarks' AND c.userId = @userId
              ORDER BY c.version DESC`,
      parameters: [{ name: '@userId', value: userId }],
    }, { partitionKey: userId }).fetchAll();
    return resources[0] || null;
  }

  // Read a named bookmarks-shared doc (e.g. "google") for nested ref resolution.
  async function getLatestShared(name) {
    if (!bookmarksContainer) return null;
    const { resources } = await bookmarksContainer.items.query({
      query: `SELECT TOP 1 * FROM c
              WHERE c.type = 'bookmarks-shared' AND c.name = @name
              ORDER BY c.version DESC`,
      parameters: [{ name: '@name', value: name }],
    }, { partitionKey: `shared:${name}` }).fetchAll();
    return resources[0] || null;
  }

  // Walk the stored menu tree; replace any { ref: "bookmarks" } node with
  // an expanded copy of the caller's bookmarks (and recursively resolve
  // any { ref: "<name>" } shared refs inside it). All resolved nodes get
  // _ref/_refVersion metadata so PUT can strip back.
  async function resolveRefs(menu, userId) {
    if (!Array.isArray(menu)) return menu;
    let bookmarksDoc = null;

    async function resolveSharedRefs(items, visited = new Set()) {
      const out = [];
      for (const item of items) {
        if (item && item.ref && Object.keys(item).filter(k => k !== '_refError').length === 1) {
          if (visited.has(item.ref) || visited.size >= 10) {
            out.push({ ...item, _refError: true });
            continue;
          }
          const sharedDoc = await getLatestShared(item.ref);
          if (!sharedDoc) {
            out.push({ ...item, _refError: true });
            continue;
          }
          const node = { ...sharedDoc.bookmarks, _ref: item.ref, _refVersion: sharedDoc.version };
          if (Array.isArray(node.children) && node.children.length > 0) {
            const nextVisited = new Set(visited); nextVisited.add(item.ref);
            node.children = await resolveSharedRefs(node.children, nextVisited);
          }
          out.push(node);
        } else if (Array.isArray(item?.children) && item.children.length > 0) {
          out.push({ ...item, children: await resolveSharedRefs(item.children, visited) });
        } else {
          out.push(item);
        }
      }
      return out;
    }

    async function walk(items) {
      const out = [];
      for (const item of items) {
        if (item && item.ref === 'bookmarks') {
          if (bookmarksDoc === null) {
            bookmarksDoc = await getLatestBookmarks(userId);
          }
          if (!bookmarksDoc) {
            out.push({ ...item, _refError: true });
            continue;
          }
          out.push({
            name: item.name,
            description: item.description,
            children: await resolveSharedRefs(bookmarksDoc.bookmarks),
            _ref: 'bookmarks',
            _refVersion: bookmarksDoc.version,
          });
        } else if (Array.isArray(item?.children) && item.children.length > 0) {
          out.push({ ...item, children: await walk(item.children) });
        } else {
          out.push(item);
        }
      }
      return out;
    }
    return walk(menu);
  }

  // Reverse of resolveRefs on PUT — drop resolved children from any node
  // tagged _ref and restore the pointer form. Read-only: edits under the
  // bookmarks subtree don't write back to the bookmarks doc.
  function stripRefs(menu) {
    if (!Array.isArray(menu)) return menu;
    return menu.map(item => {
      if (!item || typeof item !== 'object') return item;
      if (item._ref === 'bookmarks') {
        const out = { ref: 'bookmarks' };
        if (item.name) out.name = item.name;
        if (item.description) out.description = item.description;
        return out;
      }
      if (Array.isArray(item.children) && item.children.length > 0) {
        return { ...item, children: stripRefs(item.children) };
      }
      return item;
    });
  }

  // Find the latest version number for a user.
  async function getLatestVersion(userId) {
    const { resources } = await container.items.query({
      query: `SELECT TOP 1 c.version, c.updatedAt FROM c
              WHERE c.type = 'menu' AND c.userId = @userId
              ORDER BY c.version DESC`,
      parameters: [{ name: '@userId', value: userId }],
    }).fetchAll();
    return resources.length > 0 ? resources[0].version : 0;
  }

  // GET /api/menu — fetch the latest menu version
  router.get('/api/menu', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const latestVersion = await getLatestVersion(userId);

      if (latestVersion === 0) {
        return res.json({ menu: [], version: 0, updatedAt: null });
      }

      const { resource } = await container.item(versionDocId(userId, latestVersion), userId).read();
      if (!resource) {
        return res.json({ menu: [], version: 0, updatedAt: null });
      }

      const resolvedMenu = await resolveRefs(resource.menu, userId);

      res.json({
        menu: resolvedMenu,
        version: resource.version,
        updatedAt: resource.updatedAt,
      });
    } catch (error) {
      console.error('Error fetching menu:', error);
      res.status(500).json({ error: 'Failed to fetch menu', message: error.message });
    }
  });

  // PUT /api/menu — create a new version
  router.put('/api/menu', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const { menu, baseVersion } = req.body;

      if (!Array.isArray(menu)) {
        return res.status(400).json({ error: 'Request body must contain a menu array' });
      }

      // Strip resolved refs back to pointers before storing. Bookmarks live
      // in their own doc; the menu should only hold { ref } markers.
      const menuToStore = stripRefs(menu);

      const latestVersion = await getLatestVersion(userId);

      // Conflict check: if client says it was editing from version N,
      // but the latest is now > N, someone else saved in between
      if (baseVersion !== undefined && baseVersion !== latestVersion) {
        const { resource: current } = await container.item(versionDocId(userId, latestVersion), userId).read();
        return res.status(409).json({
          error: 'Conflict detected',
          message: 'Menu has been modified elsewhere.',
          currentMenu: current ? current.menu : [],
          currentVersion: latestVersion,
        });
      }

      const newVersion = latestVersion + 1;
      const now = new Date().toISOString();

      await container.items.create({
        id: versionDocId(userId, newVersion),
        userId,
        type: 'menu',
        version: newVersion,
        menu: menuToStore,
        updatedAt: now,
      });

      // Return the resolved form so the client sees the same tree shape.
      const resolvedMenu = await resolveRefs(menuToStore, userId);
      res.json({ menu: resolvedMenu, version: newVersion, updatedAt: now });
    } catch (error) {
      console.error('Error saving menu:', error);
      res.status(500).json({ error: 'Failed to save menu', message: error.message });
    }
  });

  // GET /api/menu/history — list versions (lightweight, no menu payload)
  router.get('/api/menu/history', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const limit = Math.min(parseInt(req.query.limit) || 20, 100);
      const offset = parseInt(req.query.offset) || 0;

      const { resources } = await container.items.query({
        query: `SELECT c.version, c.updatedAt FROM c
                WHERE c.type = 'menu' AND c.userId = @userId
                ORDER BY c.version DESC
                OFFSET @offset LIMIT @limit`,
        parameters: [
          { name: '@userId', value: userId },
          { name: '@offset', value: offset },
          { name: '@limit', value: limit },
        ],
      }).fetchAll();

      res.json({ versions: resources });
    } catch (error) {
      console.error('Error fetching menu history:', error);
      res.status(500).json({ error: 'Failed to fetch history', message: error.message });
    }
  });

  // GET /api/menu/history/:version — fetch a specific version
  router.get('/api/menu/history/:version', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const version = parseInt(req.params.version);

      if (isNaN(version)) {
        return res.status(400).json({ error: 'Version must be a number' });
      }

      const { resource } = await container.item(versionDocId(userId, version), userId).read();

      if (!resource) {
        return res.status(404).json({ error: 'Version not found' });
      }

      res.json({
        menu: resource.menu,
        version: resource.version,
        updatedAt: resource.updatedAt,
      });
    } catch (error) {
      if (error.code === 404) {
        return res.status(404).json({ error: 'Version not found' });
      }
      console.error('Error fetching menu version:', error);
      res.status(500).json({ error: 'Failed to fetch version', message: error.message });
    }
  });

  return router;
}
