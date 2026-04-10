import { Router } from 'express';

/**
 * Creates the AT menu routes as an Express router.
 * Menu versions are append-only documents in Cosmos DB (HomepageDB/userdata).
 * Each save creates a new version. GET returns the latest. No mutations.
 *
 * @param {{
 *   requireAuth: Function,
 *   container: import('@azure/cosmos').Container,
 * }} opts
 */
export function createATRoutes({ requireAuth, container }) {
  const router = Router();

  function versionDocId(userId, version) {
    return `menu_${userId}_v${version}`;
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

      res.json({
        menu: resource.menu,
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
        menu,
        updatedAt: now,
      });

      res.json({ menu, version: newVersion, updatedAt: now });
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
