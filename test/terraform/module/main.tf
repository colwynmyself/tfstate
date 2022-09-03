provider "local" {}

resource "local_file" "example" {
    content  = "Version 1"
    filename = "${path.module}/files/example.txt"
}
