Generalised implementation of [Ippsec's forwardshell](https://github.com/IppSec/forward-shell/tree/master).

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
