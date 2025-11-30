# Set up Docker

- Install docker engine

```
# 1. Update and install packages to allow apt to use a repo over HTTPS
sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release

# 2. Add Docker's official GPG key
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
  sudo gpg --dearmour -o /etc/apt/keyrings/docker.gpg

# 3. Set up the stable repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 4. Install Docker Engine, CLI, and containerd
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

- Start and enable docker deamon

```
# start now and enable at boot
sudo systemctl enable --now docker

# check status
sudo systemctl status docker --no-pager
```

- (Optional) allow running docker without sudo

```
sudo usermod -aG docker $USER
# then either log out & back in, or:
newgrp docker
```

- Verify

```
# quick smoke-test
docker run --rm hello-world

# optional: list installed components
docker version
docker info
```
