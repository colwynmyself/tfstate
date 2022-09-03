package main

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func main() {
	state := terraform.NewState()
	fmt.Println(state.Version)
}
