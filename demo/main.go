package main

import (
	"fmt"

	"github.com/omie/frink"
)

var queries = []string{"what is the population of Indi?",
	"who is the president of apan?",
	"how big is ussia?",
}

func main() {
	fmt.Println("Starting frink demo")
	f := frink.Frink{}
	for _, query := range queries {
		fmt.Println("Querying:", query)
		suggestion, err := f.GetSuggestion(query, true)
		if err != nil {
			fmt.Println("Error: ", err.Error())
			return
		}
		fmt.Println("Suggested: ", suggestion)
	}
}
