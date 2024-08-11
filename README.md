Generalised implementation of [Ippsec's forward-shell](https://github.com/IppSec/forward-shell/tree/master).

Typical usage:

Create a script `exploit.py` with the following content:

```python
import sys

cmdToRun = sys.argv[1]

stdout = exploitRemoteMachineAndRetrieveStdout(cmdToRun)

print(stdout)
```

This script should be able to run commands on the remote, for example:

```
# python3 exploit.py 'whoami; id'
root
uid=0(root) gid=0(root) groups=0(root)
```

Once this is set up, we can install and run forward shell.

```bash
go install github.com/stacksparrow4/fwd-shell@latest
```

```bash
fwd-shell python3 exploit.py
```

You can also customize the timing options:

```
  -cmd-delay duration
        delay between sending a command and retrieving it's output (default 500ms)
  -read-interval duration
        interval for background read loop (default 1s)
```

# Reusing a connection

`fwd-shell` will run your script many times. If you have an exploit that takes some time to start up but can run multiple seperate commands once started, you can turn it into a HTTP server and run `fwd-shell` with curl. For example, you could do:

```python
from http.server import SimpleHTTPRequestHandler, HTTPServer

class MyHandler(SimpleHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers['Content-Length'] or "0")
        data = self.rfile.read(content_length)

        self.send_response(200)
        self.end_headers()

        resp = execute_command(data)

        self.wfile.write(resp)

httpd = HTTPServer(('', 8080), MyHandler)
httpd.serve_forever()
```

And then start that script to start the HTTP server. While it is running, run fwd-shell as follows:

```bash
fwd-shell curl -X POST http://localhost:8080 --data
```
