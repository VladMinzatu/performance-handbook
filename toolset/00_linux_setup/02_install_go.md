# Install Go

- Get the latest Go version for linux arm 64 from https://go.dev/dl/
- Remove previous binary and extract new, e.g.:

```
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.25.1.linux-arm64.tar.gz
```

- Add this to the $HOME/.profile

```
export PATH=$PATH:/usr/local/go/bin

source ~/.profile
```

- Now it should be installed

```
go version
```
