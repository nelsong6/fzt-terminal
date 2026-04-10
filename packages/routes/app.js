import { Router } from 'express';

/**
 * Creates the AT menu routes as an Express router.
 *
 * @param {{
 *   requireAuth: Function,
 *   bookmarksContainerClient: import('@azure/storage-blob').ContainerClient,
 * }} opts
 */
export function createATRoutes({ requireAuth, bookmarksContainerClient }) {
  const router = Router();

  // Sanitize userId for blob naming
  function menuBlobName(userId) {
    return 'menu-' + userId.replace(/[|]/g, '_').replace(/[^a-zA-Z0-9_-]/g, '') + '.yaml';
  }

  // Read a JSON blob by name. Returns { data, lastModified } or null.
  async function readBlob(name) {
    const blob = bookmarksContainerClient.getBlobClient(name);
    const props = await blob.getProperties().catch(() => null);
    if (!props) return null;
    const download = await blob.download(0);
    const chunks = [];
    for await (const chunk of download.readableStreamBody) {
      chunks.push(chunk);
    }
    return {
      data: JSON.parse(Buffer.concat(chunks).toString('utf-8')),
      lastModified: props.lastModified,
    };
  }

  // Write a JSON blob by name. Returns { lastModified }.
  async function writeBlob(name, data) {
    const blob = bookmarksContainerClient.getBlockBlobClient(name);
    const content = JSON.stringify(data);
    await blob.upload(content, content.length, {
      blobHTTPHeaders: { blobContentType: 'application/json' },
    });
    const props = await blob.getProperties();
    return { lastModified: props.lastModified };
  }

  // GET /api/menu — fetch the full menu tree for the authenticated user
  router.get('/api/menu', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const result = await readBlob(menuBlobName(userId));
      if (!result) {
        return res.json({ menu: [], updatedAt: null });
      }
      res.json({ menu: result.data, updatedAt: result.lastModified.toISOString() });
    } catch (error) {
      console.error('Error fetching menu:', error);
      res.status(500).json({ error: 'Failed to fetch menu', message: error.message });
    }
  });

  // PUT /api/menu — save the full menu tree
  router.put('/api/menu', requireAuth, async (req, res) => {
    try {
      const userId = req.user.sub;
      const { menu, lastKnownVersion } = req.body;

      if (!Array.isArray(menu)) {
        return res.status(400).json({ error: 'Request body must contain a menu array' });
      }

      // Conflict detection
      if (lastKnownVersion) {
        const current = await readBlob(menuBlobName(userId));
        if (current && current.lastModified > new Date(lastKnownVersion)) {
          return res.status(409).json({
            error: 'Conflict detected',
            message: 'Menu has been modified elsewhere.',
            currentMenu: current.data,
            currentVersion: current.lastModified.toISOString(),
          });
        }
      }

      const { lastModified } = await writeBlob(menuBlobName(userId), menu);
      res.json({ menu, updatedAt: lastModified.toISOString() });
    } catch (error) {
      console.error('Error saving menu:', error);
      res.status(500).json({ error: 'Failed to save menu', message: error.message });
    }
  });

  return router;
}
