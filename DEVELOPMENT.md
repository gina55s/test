# Development

This is a draft guide to help people develop test itself. It will give you also some tips and tricks to debug you changes, test new features
and even test flists.

## Starting your own virtual test

Please follow instructions under [qemu](qemu/README.md) on how to run your own test in a VM

## Updating the code inside test

When you modify the code in the repo, build and start test it's gonna use all test binaries from `bin/` directory. If you need to change the code again you have 2 options:\

- Either restart the node
- or `scp` the new binary to replace the one on the vm
  - depends on what you replacing you might need to start the service manually first. You also need
  to restart the service after replacing the binary with `zinit restart <service>`

## Logs

- All the node logs can be inspected with `zinit log`
- If you have no access to test node you still can inspect the logs on `https://mon.grid.tf/`
  - You will need to request access to this. You can request access from OPS
- If you are debugging a VM. You have also multiple ways to inspect the VM logs
  - If you can ssh to the test node, inspect the logs under `/var/cache/modules/vmd/logs`
  - You also need to try out the console of your VM it can give some valuable information
- If above still didn't give you enough information to debug the issue I would recommend trying the same flist on a node that you have access to so you have better control/visibility
