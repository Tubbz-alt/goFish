#!/bin/bash
## Build Go
cd goServer || exit
go build
cd .. || exit

## Build C++
cd findFish || exit
mkdir build
cd build || exit
cmake .. && make
cd ../.. || exit

cd cameraCalibration || exit
mkdir build
cd build || exit
cmake .. && make
cd ../.. || exit