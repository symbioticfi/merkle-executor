package submission

import "strings"

func definiteRejection(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"insufficient funds", "intrinsic gas too low", "invalid sender", "transaction type not supported"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
