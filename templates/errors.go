package templates

// The shared leaf rather than a runtime: this file belongs to both builds, and
// the error page registry is one process-wide table either of them reads.
import "github.com/shibukawa/popcornweb/pwruntime"

// The framework renders one of these when a request fails and the client would
// rather have a page than a problem document. It also renders one in place of a
// page whose async boundary failed with no recover clause.
//
// The problem arrives already bounded: outside development it carries the
// status and the title only, so nothing here has to decide what is safe to
// show. Add a status to the switch and the framework starts using it.
func init() {
	pwruntime.RegisterHTMLErrorPage(func(problem pwruntime.Problem) pwruntime.HTMLFragment {
		fields := make([]string, 0, len(problem.Fields))
		for _, field := range problem.Fields {
			fields = append(fields, field.Field+": "+field.Message)
		}
		switch problem.Status {
		case 400:
			return Error400(Error400Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 401:
			return Error401(Error401Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 403:
			return Error403(Error403Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 404:
			return Error404(Error404Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 409:
			return Error409(Error409Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 413:
			return Error413(Error413Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 429:
			return Error429(Error429Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		default:
			return Error500(Error500Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		}
	})
}
