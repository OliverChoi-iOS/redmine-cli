package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"redmine-cli/internal/cache"
	"redmine-cli/internal/redmine"
)

func writeTable(w io.Writer, data any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	switch v := data.(type) {
	case map[string]any:
		if issues, ok := v["issues"].([]redmine.Issue); ok {
			writeIssuesTable(tw, issues)
		} else if projects, ok := v["projects"].([]redmine.Project); ok {
			writeProjectsTable(tw, projects)
		} else if users, ok := v["users"].([]redmine.User); ok {
			writeUsersTable(tw, users)
		} else if items, ok := v["statuses"].([]cache.NamedID); ok {
			writeNamedTable(tw, items)
		} else if items, ok := v["trackers"].([]cache.NamedID); ok {
			writeNamedTable(tw, items)
		} else if items, ok := v["priorities"].([]cache.NamedID); ok {
			writeNamedTable(tw, items)
		} else if items, ok := v["custom_fields"].([]cache.NamedID); ok {
			writeNamedTable(tw, items)
		} else {
			fmt.Fprintf(tw, "FIELD\tVALUE\n")
			for key, value := range v {
				fmt.Fprintf(tw, "%s\t%s\n", tableCell(key), tableCell(value))
			}
		}
	case redmine.Issue:
		writeIssuesTable(tw, []redmine.Issue{v})
	case redmine.Project:
		writeProjectsTable(tw, []redmine.Project{v})
	case redmine.User:
		writeUsersTable(tw, []redmine.User{v})
	case []redmine.Issue:
		writeIssuesTable(tw, v)
	case []redmine.Project:
		writeProjectsTable(tw, v)
	case []redmine.User:
		writeUsersTable(tw, v)
	case []cache.NamedID:
		writeNamedTable(tw, v)
	default:
		fmt.Fprintf(tw, "VALUE\n%s\n", tableCell(v))
	}
	return tw.Flush()
}

func writeIssuesTable(w io.Writer, issues []redmine.Issue) {
	fmt.Fprintf(w, "ID\tPROJECT\tSTATUS\tASSIGNEE\tUPDATED\tSUBJECT\n")
	for _, issue := range issues {
		assignee := ""
		if issue.AssignedTo != nil {
			assignee = issue.AssignedTo.Name
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", issue.ID, tableCell(issue.Project.Name), tableCell(issue.Status.Name), tableCell(assignee), tableCell(issue.UpdatedOn), tableCell(issue.Subject))
	}
}

func writeProjectsTable(w io.Writer, projects []redmine.Project) {
	fmt.Fprintf(w, "ID\tIDENTIFIER\tNAME\n")
	for _, project := range projects {
		fmt.Fprintf(w, "%d\t%s\t%s\n", project.ID, tableCell(project.Identifier), tableCell(project.Name))
	}
}

func writeUsersTable(w io.Writer, users []redmine.User) {
	fmt.Fprintf(w, "ID\tLOGIN\tNAME\tMAIL\n")
	for _, user := range users {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", user.ID, tableCell(user.Login), tableCell(user.FullName()), tableCell(user.Mail))
	}
}

func writeNamedTable(w io.Writer, items []cache.NamedID) {
	fmt.Fprintf(w, "ID\tNAME\tIDENTIFIER\n")
	for _, item := range items {
		fmt.Fprintf(w, "%d\t%s\t%s\n", item.ID, tableCell(item.Name), tableCell(item.Identifier))
	}
}

func tableCell(value any) string {
	return sanitizeTerminalCell(fmt.Sprint(value))
}

func sanitizeTerminalCell(value string) string {
	var b []rune
	runes := []rune(value)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\x1b' {
			if i+1 < len(runes) {
				switch runes[i+1] {
				case '[':
					i += 2
					for i < len(runes) && !(runes[i] >= 0x40 && runes[i] <= 0x7e) {
						i++
					}
				case ']':
					i += 2
					for i < len(runes) {
						if runes[i] == '\a' {
							break
						}
						if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '\\' {
							i++
							break
						}
						i++
					}
				}
			}
			continue
		}
		if r == '\t' || r == '\n' || r == '\r' {
			b = append(b, ' ')
			continue
		}
		if r < 0x20 || r == 0x7f || r >= 0x80 && r <= 0x9f {
			continue
		}
		b = append(b, r)
	}
	return string(b)
}
