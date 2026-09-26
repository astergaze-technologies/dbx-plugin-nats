# Vendored DBX Go plugin SDK

Copy of [`plugins/sdk/go/dbx-plugin-sdk`](https://github.com/t8y2/dbx/tree/99d80f7a83c2423d0fa572b246bf7d74044a4cbc/plugins/sdk/go/dbx-plugin-sdk)
from t8y2/dbx at commit `99d80f7a83c2423d0fa572b246bf7d74044a4cbc`, licensed Apache-2.0 (see `LICENSE`). Unmodified.

The SDK is not published as a fetchable Go module version, so `backend/go.mod`
points at this copy with a `replace`. `dbx-plugin package` overrides that replace
with the SDK bundled in the DBX plugin CLI, so released packages use the official copy.
To update: re-download both files from the commit you want and update the hash above.
