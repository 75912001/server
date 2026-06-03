#!/bin/bash

currentPath=$(realpath "$(dirname "$0")")
projectPath=$(dirname "${currentPath}")
echo "projectPath:${projectPath}"

cd "${projectPath}/bin"
./robot.exe
cd -

exit 0
