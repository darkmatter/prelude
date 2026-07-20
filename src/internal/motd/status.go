package motd

// resolveStatusItem sets Level and Status on a badge from a check outcome.
// Diagnostics for post-banner lines are no longer collected (D2).
func resolveStatusItem(item *HeaderStatus, ok bool, out string) []string {
	if ok {
		item.Level = "success"
		switch item.Output {
		case "light":
			item.Status = ""
		default:
			switch {
			case item.Ok != "":
				item.Status = item.Ok
			case out != "":
				item.Status = firstLine([]byte(out))
			default:
				item.Status = "ok"
			}
		}
		return nil
	}

	item.Level = "error"
	if item.FailLevel == "warning" {
		item.Level = "warning"
	}
	switch item.Output {
	case "light":
		item.Status = ""
	default:
		switch {
		case item.Fail != "":
			item.Status = item.Fail
		case out != "":
			item.Status = firstLine([]byte(out))
		default:
			item.Status = "fail"
		}
	}
	return nil
}
