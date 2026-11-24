package report

type Row struct { Amount int }

func Sum(rows []Row) int {
    t := 0
    for _, r := range rows { t += r.Amount }
    return t
}
