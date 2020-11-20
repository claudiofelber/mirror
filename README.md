mirror
======
The `mirror` tool mirrors a local directory tree to a remote server via SFTP. Use it in cases where rsync doesn't work or might be too complicated to get running (like on some Windows systems).

Usage
-----
`mirror [OPTIONS] localPath user[:password]@remoteHost[:port]/path`

The options are:  
`-h` `--help` Shows a help message  
`-x` `--exclude=PATTERN` Excludes file(s) from mirroring  
`-i` `--ignore=PATTERN` Ignores file(s) on remote  
`-s` `--simulate` Does not modify anything but displays the actions that would have to be performed to sync the two directories

There are two types of patterns:

* **Segment pattern:** Matches the last segment of a path only (i.e. the filename or last directory). Examples:
 * `.svn` Every Subversion folder in any directory
 * `*.gif` Every GIF file in any directory
* **Path pattern:** Matches the whole path. A path pattern always starts with a slash. Examples:
 * `/tmp` The tmp folder in the root of your local project directory
 * `/**/images/*.gif` Every GIF file in every images folder

Patterns can contain two types of wildcards:

* `*` Matches anything within a path segment
* `**` Matches anything including directory separators `/` and `\`.

Examples
--------
`mirror c:\projects\foo foo:bar@foo.com:public_html`  
Forward syncs all files in the local directory `c:\projects\foo` to the remote directory `public_html` in your home directory on the server `foo.com` (credentials are username `foo` and password `bar`). Files and directories that no longer exist in the local directory are removed from the server.

`mirror c:\projects\foo foo:bar@foo.com:public_html -x .DS_Store -x .git* -x /tmp -i /cache`  
Does not copy `.DS_Store` files and every file starting with `.git` as well as the folder `tmp` your local directory. Furthermore the remote folder `/cache` is not touched (and not removed) although it might not be available locally.  
**Note:** Depending on the shell you are using (e.g. bash) you need to quote wildcard parameters to prevent them from being expanded: `-x ".git*"`.
