package find

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func ExampleFind() {
	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(5*time.Minute),
	)
	defer cancel()

	where := "path/to/the/source"

	// Since recursive search does not active by default,
	// Find will search for files and folders only in the
	// root of where.
	results, err := Find(ctx, where, "*template*")
	if err != nil {
		log.Println(err)

		return
	}

	for _, r := range results {
		fmt.Println(r)
	}
}

func ExampleFind_withOptions() {
	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(5*time.Minute),
	)
	defer cancel()

	where := "path/to/the/source"

	// Results will contain only folders searched recursively.
	results, err := Find(
		ctx,
		where,
		"*template*",
		Only(Folder),
		Recursively,
	)
	if err != nil {
		log.Println(err)

		return
	}

	for _, r := range results {
		fmt.Println(r)
	}
}

func ExampleTemplate_standalone() {
	template := NewTemplate("*custom*&*template*")

	// Can be any string slice, resulted from different sources.
	s, err := os.ReadDir("some/folder")
	if err != nil {
		log.Fatalln(err)
	}

	for _, el := range s {
		if template.Match(el.Name()) {
			// Do something here...
		}
	}
}

func ExampleTemplates() {
	ts := []string{"*this*", "*that*"}

	templates := NewTemplates(ts)

	// Can be any string slice, resulted from different sources.
	s, err := os.ReadDir("some/folder")
	if err != nil {
		log.Fatalln(err)
	}

	for _, el := range s {
		if MatchAny(templates, el.Name()) {
			// Do something here if match any of the templates...
		}

		if MatchAll(templates, el.Name()) {
			// Do something here if match all of the templates...
		}
	}
}
