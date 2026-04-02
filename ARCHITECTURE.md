# Sriracha Architecture
[![Donate](https://img.shields.io/liberapay/receives/rocket9labs.com.svg?logo=liberapay)](https://liberapay.com/rocket9labs.com)

## Layout

The source code of Sriracha is organized as follows:

| Directory | Synopsis |
| --        | --       |
| [sriracha](https://codeberg.org/tslocum/sriracha) (root) | Imported by plugins to interact with the database. |
| [internal/database](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/database) | Provides methods for interacting with the database. |
| [internal/server](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/server) | Sriracha web server. |
| [internal/server/locale](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/server/locale) | [Gettext](https://en.wikipedia.org/wiki/Gettext) locale files. |
| [internal/server/template](https://codeberg.org/tslocum/sriracha/src/branch/main/internal/server/template) | [Go HTML](https://pkg.go.dev/html/template) template files.
| [model](https://codeberg.org/tslocum/sriracha/src/branch/main/model) | Sriracha data types. |
| [util](https://codeberg.org/tslocum/sriracha/src/branch/main/util) | Sriracha utility functions and variables. |

## Design

Sriracha maintains a [PostgreSQL](https://www.postgresql.org) connection pool containing one connection.

When initializing a new Sriracha database, the version 1 schema is applied to the database, then upgraded to version 2, and so on.

When upgrading an existing Sriracha database, schema changes are applied automatically.

When a web request is received, a database connection is obtained from the connection pool before processing the request.

This connection will typically be held until the server finishes processing the request.

Some requests will release the database connection early, but only when it is safe to do so.

This allows Sriracha to safely handle multiple web requests simultaneously.

Whenever Sriracha data is modified, static HTML files are written to the configured root directory.

When a static file server is used in conjunction with Sriracha, visitors will only connect to
the static file server unless they create or delete a post.

Because of this, it is possible to make use of extensive server-side and client-side caching.

## Format

Sriracha posts and threads are formatted to be easily readable by humans and machines.

Each post is formatted as follows:

```html
<div class="thread">
    <div id="1" class="op post">
        <div id="post1">
            <label class="attachmentinfo">
                <span class="filesize">File: <a href="/b/src/1774828791716.png" target="_blank" onclick="return expandFile(event, '1');">1774828791716.png</a>&ndash;(92KB, 1920x1080, file_original_1.png)</span><br>
            </label>
            <div id="thumbfile1">
                <a href="/b/src/1774828791716.png" target="_blank" onclick="return expandFile(event, '1');"><img src="/b/thumb/1774828791716s.png" alt="1774828791716.png" class="thumb" id="thumbnail1" width="250" height="140"></a>
            </div>
            <div id="file1" class="thumb" style="display: none;" data-expand="&lt;a href=&#34;/b/src/1774828791716.png&#34; onclick=&#34;return expandFile(event, &#39;1&#39;);&#34;&gt;&lt;img src=&#34;/b/src/1774828791716.png&#34; width=&#34;1920&#34; height=&#34;1080&#34; style=&#34;pointer-events: inherit;&#34;&gt;&lt;/a&gt;"></div>
            <label class="postinfo">
                <input type="checkbox" name="delete[]" value="1">
                <span class="filetitle">Thread subject text</span>
                <span class="postername">Anonymous</span><span class="postertrip">!XgzPoOaLlE</span> tGQV5 <time datetime="2026-03-29T23:59:51Z" title="2026-03-29T23:59:51Z">2026/03/29<wbr>(Sun)<wbr>16:59:51</time>
                <span class="reflink">
                    <span class="postui">
                        <a href="/sriracha/?action=report&board=2&post=1" title="Report">R</a>
                    </span>
                    <a href="/b/res/1.html#1">No.</a><a href="/b/res/1.html#q1">1</a>
                </span>
            </label>
            <span class="threadui">&nbsp;[<a href="/b/res/1.html">Reply</a>]</span>
            <div class="message">
                Thread message text.
            </div>
            <p style="display: none;"></p>
        </div>
    </div>
    <table>
        <tbody>
            <tr>
                <td class="doubledash"></td>
                <td class="reply post" id="2">
                    <div id="post2">
                        <label class="postinfo">
                            <input type="checkbox" name="delete[]" value="2">
                            <span class="postername">Anonymous</span> tGQV5 <time datetime="2026-03-30T00:00:10Z" title="2026-03-30T00:00:10Z">2026/03/29<wbr>(Sun)<wbr>17:00:10</time>
                            <span class="reflink">
                                <span class="postui">
                                    <a href="/sriracha/?action=report&board=2&post=2" title="Report">R</a>
                                </span>
                                <a href="/b/res/1.html#2">No.</a><a href="/b/res/1.html#q2">2</a>
                            </span>
                        </label>
                        <br>
                        <label class="attachmentinfo">
                            <span class="filesize">File: <a href="/b/src/1774828810920.png" target="_blank" onclick="return expandFile(event, '2');">1774828810920.png</a>&ndash;(2MB, 1920x1080, file_original_2.png)</span><br>
                        </label>
                        <div id="thumbfile2">
                            <a href="/b/src/1774828810920.png" target="_blank" onclick="return expandFile(event, '2');"><img src="/b/thumb/1774828810920s.png" alt="1774828810920.png" class="thumb" id="thumbnail2" width="250" height="140"></a>
                        </div>
                        <div id="file2" class="thumb" style="display: none;" data-expand="&lt;a href=&#34;/b/src/1774828810920.png&#34; onclick=&#34;return expandFile(event, &#39;2&#39;);&#34;&gt;&lt;img src=&#34;/b/src/1774828810920.png&#34; width=&#34;1920&#34; height=&#34;1080&#34; style=&#34;pointer-events: inherit;&#34;&gt;&lt;/a&gt;"></div>
                        <div class="message">
                            Reply message text.
                        </div>
                        <p style="display: none;"></p>
                    </div>
                </td>
            </tr>
        </tbody>
    </table>
</div>
```

Each thread is formatted as follows:

```html
<div class="thread">
    <div id="1" class="op post">
        <!-- Snip -->
    </div>
    <table>
        <tbody>
            <tr>
                <td class="doubledash"></td>
                <td class="reply post" id="2">
                    <!-- Snip -->
                </td>
            </tr>
        </tbody>
    </table>
    <table>
        <tbody>
            <tr>
                <td class="doubledash"></td>
                <td class="reply post" id="3">
                    <!-- Snip -->
                </td>
            </tr>
        </tbody>
    </table>
</div>
```

The `datetime` attribute of a post's `time` element specifies the UTC date/time when the post was created.

| Class | Contents |
| --    | --       |
| post | Thread or reply post |
| op | Thread post |
| reply | Reply post |
| postinfo | Subject, name, tripcode, date/time and reflink  |
| filetitle | Subject |
| postername | Name |
| postertrip | Tripcode |
| attachmentinfo | File size, dimensions and original name |
| thumbfile### | Thumbnail |
| file### | Expansion HTML (via data-expand) |
