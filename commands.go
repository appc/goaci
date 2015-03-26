package main

var (
	commandsHash map[string]command = make(map[string]command)
)

func init() {
	commands := []command{
		newBuilderCommand(newGoParameterMapper()),
		newBuilderCommand(newCmakeParameterMapper()),
	}
	for _, c := range commands {
		commandsHash[c.Name()] = c
	}
}
