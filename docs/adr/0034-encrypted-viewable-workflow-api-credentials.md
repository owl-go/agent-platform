# Encrypt and allow owners to view Workflow API credentials

Workflow API Key/API Secret pairs are now encrypted at rest and may be read only by the owning User through the Workflow detail API, because the product requires persistent display and copy support. The API Secret remains outside ordinary Workflow snapshots and runtime output; regeneration replaces the encrypted value, verifier, and derived-token signing material, while existing credentials created before ciphertext storage require regeneration.
