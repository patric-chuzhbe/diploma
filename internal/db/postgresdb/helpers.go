package postgresdb

func toInterfaceSlice(strSlice []string) []interface{} {
	result := make([]interface{}, len(strSlice))
	for i, v := range strSlice {
		result[i] = v
	}
	return result
}
