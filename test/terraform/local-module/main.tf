provider "local" {}

resource "local_file" "example" {
    content  = "Version 1"
    filename = "${path.module}/files/example.txt"
}

resource "local_file" "anotherone" {
    content  = "Version 2"
    filename = "${path.module}/files/example2.txt"
}
