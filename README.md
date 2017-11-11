# matrix-media-repo
A domain-aware media repository for matrix

# Installing

Assuming Go 1.9 and JDK 1.8 are already installed on your PATH:
```
# Get it
git clone https://github.com/turt2live/matrix-media-repo
cd matrix-media-repo

# Build it
go get github.com/constabulary/gb/...
gb vendor restore
gb build

# Run it
bin/matrix-media-repo
```
