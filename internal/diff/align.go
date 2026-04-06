package diff

import "github.com/joshuazd/nit/internal/models"

// AlignHunkLines pairs remove/add lines for side-by-side display.
func AlignHunkLines(lines []models.DiffLine) []models.SideBySideRow {
	var rows []models.SideBySideRow
	var removes []models.DiffLine
	var adds []models.DiffLine

	flush := func() {
		n := len(removes)
		if len(adds) > n {
			n = len(adds)
		}
		for i := range n {
			var left, right *models.DiffLine
			if i < len(removes) {
				l := removes[i]
				left = &l
			}
			if i < len(adds) {
				r := adds[i]
				right = &r
			}
			rows = append(rows, models.SideBySideRow{Left: left, Right: right, RowType: models.RowChange})
		}
		removes = removes[:0]
		adds = adds[:0]
	}

	for _, dl := range lines {
		switch dl.LineType {
		case models.LineHunkHeader:
			flush()
			d := dl
			rows = append(rows, models.SideBySideRow{Left: &d, Right: nil, RowType: models.RowHunkHeader})
		case models.LineRemove:
			if len(adds) > 0 && len(removes) == 0 {
				flush()
			}
			removes = append(removes, dl)
		case models.LineAdd:
			adds = append(adds, dl)
		default:
			// Context line
			flush()
			d := dl
			rows = append(rows, models.SideBySideRow{Left: &d, Right: &d, RowType: models.RowContext})
		}
	}

	flush()
	return rows
}
