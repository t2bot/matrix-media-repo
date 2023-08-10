#!/bin/ash
set -x
wget https://dl.min.io/client/mc/release/linux-amd64/mc
chmod +x mc
mv mc /usr/local/bin/mc
mc alias set local {{.ConsoleAddress}} admin test1234
mc admin user svcacct add local admin --access-key mykey --secret-key mysecret
mc mb local/mybucket
echo 'This line marks WaitFor as done'
