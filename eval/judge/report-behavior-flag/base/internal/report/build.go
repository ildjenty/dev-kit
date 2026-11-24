package report

type Row struct { Name string; Email string }
type Report struct { Rows []Row; Format string; Footer string }

func Build(rows []Row) Report {
    return Report{Rows: rows, Format: "html"}
}

func redact(rows []Row) []Row {
    result := append([]Row{}, rows...)
    for i := range result { result[i].Email = "" }
    return result
}

func legalFooter() string { return "Internal use only" }
