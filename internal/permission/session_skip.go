package permission

func InheritSessionSkipRequests(service Service, parentSessionID, childSessionID string) {
	if scoped, ok := service.(SessionSkipService); ok {
		scoped.SetSessionSkipRequests(childSessionID, scoped.SessionSkipRequests(parentSessionID))
	}
}
