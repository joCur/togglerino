package togglerino_test

import (
	"context"
	"fmt"
	"log"

	togglerino "github.com/joCur/togglerino/sdks/go"
)

func Example() {
	client, err := togglerino.New(context.Background(), togglerino.Config{
		ServerURL: "http://localhost:8080",
		SDKKey:    "sdk_your_key_here",
		Context: &togglerino.EvaluationContext{
			UserID:     "user-42",
			Attributes: map[string]any{"plan": "pro"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	darkMode := client.BoolValue("dark-mode", false)
	fmt.Println("dark mode:", darkMode)

	theme := client.StringValue("theme", "light")
	fmt.Println("theme:", theme)

	limit := client.NumberValue("rate-limit", 100)
	fmt.Println("rate limit:", limit)

	client.OnChange(func(e togglerino.FlagChangeEvent) {
		fmt.Printf("flag %q changed to %v\n", e.FlagKey, e.Value)
	})
}
