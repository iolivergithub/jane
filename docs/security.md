# Security

Read carefully:

   * A self-signed key is provided called temporary.key/crt - DO NOT USE THIS IN PRODUCTION OR ANYWHERE. Browsers will complain if you use this.
   * THIS IS NOT SECURE!!!
   * PUTTING PRIVATE KEYS ON GITHUB FOR ANYTHING ELSE THAN A DEMONSTRATION IS CRAZY. DO NOT DO THIS.
   * TO SAVE YOURSELF, SET THE use http FIELDS TO true in the configuration file.
   * That isn't secure either...  
   * GENERATE YOUR OWN KEYS AND KEEP THEM SECURE and use https
   * Tarzan runs over HTTP !!!!   
   * DO NOT USE Tarzan IN UNSAFE MODE !!! (Even if you're not root!)
   * Jane signs the footers of claims, results and sessions (hash, signature, key id and algorithm). Set `signing: keyfile:` in the configuration file to a PEM private key of your own; without it a random key is generated each time Jane starts, is not recorded anywhere, and those footers can't be verified once Jane stops. Keeping and distributing the keys and key ids is up to you, and there is no verifier yet. There is some code to talk PKCS#11 (tested with a YubiHSM) but it isn't used.
   * Logins can be required for the Web UI, the REST API and the x3270 interface - see [Authentication](authentication.md). They are all off by default. A login over plain HTTP, and any x3270 logon, sends the password unencrypted.
   * The x3270 interface (tn3270, port 3270) is never encrypted. Bind it to 127.0.0.1 with `x3270: listenon:` and use an SSH tunnel, or firewall it.
   * Use `listenon` in the `rest:`, `web:` and `x3270:` sections to bind each service to one address (e.g. 127.0.0.1) instead of all interfaces.
   * The deb package runs Jane as the unprivileged `jane` user. Keys must be readable by group `jane`, and nobody else.
   * Did I tell you that the temporary.key and temporary.crt have been on github and gitlab pages for years; also in the forks and elsewhere...?

Now read the section on the use of this software in a production environment.

# Use in a Production Environment

Don't.  This is not secure and many points where errors and exceptions should be captured are not implemented.

You are welcome to contribute code to fix this