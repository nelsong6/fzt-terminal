import { Router } from 'express';

/**
 * Creates the AT menu routes as an Express router.
 * Menu data lives in Cosmos DB (HomepageDB/userdata) with version history.
 *
 * @param {{
 *   requireAuth: Function,
 *   container: import('@azure/cosmos').Container,
 * }} opts
 */
export function createATRoutes({ requireAuth, container }) {
  const router = Router();

  function menuId(userId) {
    return `menu_${userId}`;
  }

  function historyId(userId, version) {
    return `menu_${userId}_v${version}`;
  }

  // GET /api/menu — fetch the current menu tree
  router.get('/api/menu', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const { resource } = await container.item(menuId(userId), userId).read();

      if (!resource) {
        return res.json({ menu: [], updatedAt: null, etag: null });
      }

      res.json({
        menu: resource.menu,
        updatedAt: resource.updatedAt,
        etag: resource._etag,
      });
    } catch (error) {
      if (error.code === 404) {
        return res.json({ menu: [], updatedAt: null, etag: null });
      }
      console.error('Error fetching menu:', error);
      res.status(500).json({ error: 'Failed to fetch menu', message: error.message });
    }
  });

  // PUT /api/menu — save the menu tree, creating a version history entry
  router.put('/api/menu', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const { menu, lastKnownVersion } = req.body;

      if (!Array.isArray(menu)) {
        return res.status(400).json({ error: 'Request body must contain a menu array' });
      }

      // Read current doc for version number
      let currentVersion = 0;
      let currentDoc = null;
      try {
        const { resource } = await container.item(menuId(userId), userId).read();
        if (resource) {
          currentDoc = resource;
          currentVersion = resource.version || 0;
        }
      } catch (e) {
        if (e.code !== 404) throw e;
      }

      const newVersion = currentVersion + 1;
      const now = new Date().toISOString();

      // Build updated current document
      const newDoc = {
        id: menuId(userId),
        userId,
        type: 'menu',
        menu,
        version: newVersion,
        updatedAt: now,
      };

      // Upsert with etag conflict detection if provided
      const options = {};
      if (lastKnownVersion) {
        options.accessCondition = { type: 'IfMatch', condition: lastKnownVersion };
      }

      try {
        const { resource } = await container.items.upsert(newDoc, options);

        // Write history document for the previous state AFTER successful upsert
        if (currentDoc) {
          await container.items.upsert({
            id: historyId(userId, currentVersion),
            userId,
            type: 'menu-history',
            menu: currentDoc.menu,
            version: currentVersion,
            updatedAt: currentDoc.updatedAt,
          });
        }

        res.json({
          menu: resource.menu,
          updatedAt: resource.updatedAt,
          etag: resource._etag,
          version: resource.version,
        });
      } catch (error) {
        if (error.code === 412) {
          // Etag mismatch — someone else wrote, no history written
          const { resource: current } = await container.item(menuId(userId), userId).read();
          return res.status(409).json({
            error: 'Conflict detected',
            message: 'Menu has been modified elsewhere.',
            currentMenu: current.menu,
            currentVersion: current._etag,
          });
        }
        throw error;
      }
    } catch (error) {
      console.error('Error saving menu:', error);
      res.status(500).json({ error: 'Failed to save menu', message: error.message });
    }
  });

  // GET /api/menu/history — list version history (lightweight, no menu payload)
  router.get('/api/menu/history', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const limit = Math.min(parseInt(req.query.limit) || 20, 100);
      const offset = parseInt(req.query.offset) || 0;

      const { resources } = await container.items.query({
        query: `SELECT c.version, c.updatedAt FROM c
                WHERE c.type = 'menu-history' AND c.userId = @userId
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

  // GET /api/menu/history/:version — fetch a specific historical version
  router.get('/api/menu/history/:version', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const version = parseInt(req.params.version);

      if (isNaN(version)) {
        return res.status(400).json({ error: 'Version must be a number' });
      }

      const { resource } = await container.item(historyId(userId, version), userId).read();

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
