package autoload

/*
	You can just read the .env file on import just by doing

		import _ "github.com/SpherePrime/CLI/vendordeps/joho/godotenv/autoload"

	And bob's your mother's brother
*/

import "github.com/SpherePrime/CLI/vendordeps/joho/godotenv"

func init() {
	godotenv.Load()
}
