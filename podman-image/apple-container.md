```bash
container build -t podman-machine-os:6.2 -f podman-image/Containerfile.APPLECONTAINER podman-image
container machine create podman-machine-os:6.2 --name podman-machine-os --cpus 4 --memory 4G
container machine run podman run hello
podman --connection apple-container run hello
```
