# Terraform State Viewer

Mostly a toy repo right now. The first feature I'd like to build into it is the ability to compare two tfstate files
fairly easily on the CLI.

Psuedocode

```txt
check that the current folder is a terraform module

list tfstate versions:
  if local:
    check terraform.tfstate and terraform.tfstate.backup
  if s3:
    if versioned:
      fetch all object versions and preserve order
    else:
      exit, we can't do anything

for each tfstate version:
  terraform show -json STATEFILE
  create tfstate representation
  create delta with previous version

  if correct tfstate version:
    if local:
      terraform state push terraform.state.backup -force
    if s3:
      terraform state push STATEFILE -force

helper functions:
  display tfstate delta
  display tfstate
```
