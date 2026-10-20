# addjaneuser

Adds a user to Jane, or sets a new password for an existing one. Users are needed when the Web UI login (`auth: web: true`) or REST basic auth (`auth: rest: true`) is enabled in `config.yaml`; see [janeserver/auth/README.md](../../janeserver/auth/README.md).

The Jane deb package installs it as `/opt/jane/addjaneuser`, with this README at `/usr/share/doc/janeattestationengine/addjaneuser.md`. The package only *recommends* the Python packages below, so install them if they were skipped. The packaged `config.yaml` uses the database name `asvr`.

## Requirements

- Python 3
- [argon2-cffi](https://pypi.org/project/argon2-cffi/), to hash the password
- [pymongo](https://pypi.org/project/pymongo/), to write the user to Jane's MongoDB database
- Network access to that MongoDB database

Install the packages with your distribution's package manager:

```
apt install python3-argon2 python3-pymongo
```

or with pip, using the `requirements.txt` in this directory:

```
pip install -r requirements.txt
```

## Usage

```
addjaneuser [--dbuser NAME --dbpass PASSWORD] USERNAME MONGODB_URL DATABASE
```

`MONGODB_URL` and `DATABASE` are the `database: connection` and `database: name` values from Jane's `config.yaml`, for example:

```
./addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

The password is asked for twice and never appears on the command line. To run it non-interactively, pipe the password in on standard input:

```
echo 'secret' | ./addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

`./addjaneuser -h` prints the full help.

## When MongoDB requires a login

If MongoDB has authorization enabled, addjaneuser needs a MongoDB account. It reads the same environment variables as Jane:

| Variable | Meaning |
|---|---|
| `JANE_MONGODB_USERNAME` | MongoDB user; unset or empty means no account |
| `JANE_MONGODB_PASSWORD` | its password |
| `JANE_MONGODB_AUTHSOURCE` | the database the user was created in (default `admin`) |

On a deb install they are in `/etc/opt/jane/jane.env`. As root, load them and run the script:

```
set -a; . /etc/opt/jane/jane.env; set +a
/opt/jane/addjaneuser ian mongodb://127.0.0.1:27017 asvr
```

`--dbuser NAME --dbpass PASSWORD` (both together) override the variables. A password on the command line is visible to other users (`ps`) and stays in your shell history; that is your choice to make, the variables are the safer way.

## Notes

- If the user does not exist it is created; if it does, its password is replaced. The old password is not needed, so anyone who can run this script against the database can reset any password.
- Users are stored in the `users` collection with the password as an Argon2id hash in PHC format (19 MiB memory, 2 iterations, 1 lane). Jane only ever verifies these hashes; it never stores passwords itself.
- Passwords longer than 1024 bytes are rejected, as Jane rejects them at login.
- There is no command to delete a user; remove the document from the `users` collection, e.g. in `mongosh`: `db.users.deleteOne({username: "ian"})`.
- Jane does not need to be restarted after a user is added or changed.
