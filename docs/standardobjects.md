# Standard opaque objects

Opaque objects are values Jane can explain wherever it shows them: a `value` (for example a hash, a GUID or a vendor ID) with a `type`, a short and a long description. The Web UI looks values up in the opaque objects (for example in PCR claims) and shows their descriptions.

Since 1.5.6 (#102) there is a published set, like the [standard intents](standardintents.md): the file https://github.com/iolivergithub/jane/blob/main/etc/standardobjects/standardobjects.json, a JSON list of opaque objects:

```json
[
  { "value": "IFX", "type": "tpm-manufacturer", "shortdescription": "Infineon", "longdescription": "TPM manufacturer ID of Infineon" }
]
```

The published list starts empty; entries are added over time.

## Loading

In the Web UI, **Opaque objects → Load standard objects** (or *New → Load/Reload Standard Opaque Objects*) downloads the file and **adds or updates** each object by its `value`:

- a downloaded object **overwrites** an existing one with the same value, whether it was loaded before or entered by hand, so corrections to the published set reach every Jane at the next load;
- objects whose value is not in the download (local additions) are **never** changed or removed.

The page then says how many objects were loaded; an empty published list gives "nothing to load". If the download fails (network or TLS error, HTTP status, bad JSON), the page says so and the reason is in Jane's log. The download uses normal TLS verification, as for the standard intents.

The jane package installs a copy matching the installed version as `/etc/opt/jane/standardobjects.json` (also in the Windows zip), for loading by hand, e.g. with `mongoimport` or by entering the objects in the Web UI.
