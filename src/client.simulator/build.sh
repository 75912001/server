#!/bin/bash

scriptDir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
serverPath=$(cd "${scriptDir}/../.." && pwd)
simulatorPath="src/client.simulator"

echo "serverPath:${serverPath}"
echo "simulatorPath:${serverPath}/${simulatorPath}"

cd "${serverPath}" || exit 1
export GOCACHE="${GOCACHE:-${serverPath}/.gocache}"

go build -buildvcs=false -o "${simulatorPath}/bin/client.simulator.exe" ./${simulatorPath}/main
if [ $? -ne 0 ]; then
    echo -e "\e[91m build client.simulator failed.\e[0m"
    exit 1
fi

echo -e "\e[92m build client.simulator successfully.\e[0m"
exit 0
