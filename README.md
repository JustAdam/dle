DLE (docker logentries)
-----------------------

A client to send your Docker logs to logentries.com.

## Requirements

- Docker API v 1.12 (if you have an older version of Docker, you can use tag 0.0.2)

## First steps

- Configure all your containers to log to stdout/stderr
- Set up log rotation on your Docker logs (not actually required)

## Next

### Logentries.com

- Log into your account at logentries.
- Add a new host and select manual configuration.
- Give your log a relevant name (*Docker catchall*)
- Select Token TCP
- Register new log
- The token is needed by `dle`. Pass it either by the environment variable `DLE_DEFAULT_TOKEN` or the flag `--default-token`

- Start dle (we recomend using our Docker image)

#### Docker image

```
$ docker run -d --name dle -e DOCKER_ENDPOINT=unix:///tmp/docker.sock -e DLE_DEFAULT_TOKEN=YOUR_DEFAULT_TOKEN -e DLE_LOG_LEVEL=warn -v /var/run/docker.sock:/tmp/docker.sock justadam/dle:latest
```

> You can also pass `-e DLE_IGNORE=true` to ignore this container's logging, or `-e DLE_TOKEN=token` to send the log to a specific log at logentries.

### Configuration

Configuration is done via the environment.  Once `dle` is running it will find all active containers and forward their logs to the default log you just created at logentries.  That is; all logs will be collected in a single entry but this is probably not what you want.

#### One log entry for each container

- Log into logentries
- Click Add log in the host you created earlier.
- Give the log a name
- Select Token TCP
- Register new log
- Copy the token
- Start your container and specify the environment variable `DLE_TOKEN=YOUR-TOKEN`
- Repeat for each container

```
$ docker run -d --name your-container -e DLE_TOKEN=YOUR-TOKEN you/container:latest
```

`dle` will see this new container and start forwarding the logs.

#### Ignoring containers

- When creating your container specify the environment variable `DLE_IGNORE=true`

```
$ docker run -d --name your-container -e DLE_IGNORE=true you/container:latest
```

## Building

- Check out this repository
- Install gvp and gpm (used for package management)

```
$ gvp init
$ source gvp in
$ gpm install
$ go build -ldflags="-s" -o dle dle.go tls.go
$ chmod +x dle
$ mv dle /usr/bin/dle
```
