#!/bin/bash
set -ex
sudo apt-get install -y git cmake make pkg-config libx265-dev libde265-dev libjpeg-dev libtool
git clone https://github.com/strukturag/libheif.git
cd libheif
mkdir build
cd build
cmake ..
make
sudo make install
sudo ldconfig
