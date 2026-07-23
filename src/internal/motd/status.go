package motd

// resolveStatusItem turns a check outcome into the status text and level that
// would be cached or rendered. It is pure: no mutation of the HeaderStatus.
func resolveStatusItem(item HeaderStatus, ok bool, out string) (status, level string) {
	if ok {
		level = "success"
		switch item.Output {
		case "light":
			status = ""
		default:
			switch {
			case item.Ok != "":
				status = item.Ok
			case out != "":
				status = firstLine([]byte(out))
			default:
				status = "ok"
			}
		}
		return status, level
	}

	level = "error"
	if item.FailLevel == "warning" {
		level = "warning"
	}
	switch item.Output {
	case "light":
		status = ""
	default:
		switch {
		case item.Fail != "":
			status = item.Fail
		case out != "":
			status = firstLine([]byte(out))
		default:
			status = "fail"
		}
	}
	return status, level
}
