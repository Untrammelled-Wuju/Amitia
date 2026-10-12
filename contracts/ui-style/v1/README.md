# Amitia UI Style Patch v1

`style-patch.schema.json` defines the cross-platform style override contract used by the backend, Flutter mobile client, and Electron desktop client.

The current application UI remains the default style source. AI and user changes are sparse patches applied over that default. A reset removes the active patch. An undo restores the previous revision.

Supported patch groups:

- Theme mode, accent color, font scale, corner style, motion, and custom palette.
- Semantic color tokens.
- Typography, spacing, radius, icon, and component tokens.
- Component variants.
- Background color, gradient, artifact-backed image, or artifact-backed video.

The patch never changes page structure, navigation, business logic, or native system UI.
