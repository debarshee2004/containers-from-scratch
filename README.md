# Containers from scratch

Check out the [source video](https://youtu.be/8fi7uSYlOdc?si=6YnREwZZdOFamSMP) where they discuss how to design a container and how to implement it from ground.

Play along by downloading the code (will only work on Unix/Linux system):

```bash
git clone https://github.com/debarshee2004/containers-from-scratch.git
cd containers-from-scratch
```

Running the application:

```bash
go build main.go
```

```bash
# Run a bash shell in the container
./container run /bin/bash

# Run a specific command
./container run /bin/sh -c 'echo Hello from container'

# Show help
./container help
```
