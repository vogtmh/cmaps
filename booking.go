package main

import (
	"net/http"
)

// handleRestBooking serves /rest/booking?mode=book|remove|list. User identity is
// taken from the session, never from the request, matching the PHP behaviour.
func (app *App) handleRestBooking(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("mode")
	bookDate := q.Get("bookdate")
	bookMap := q.Get("bookmap")
	bookDesk := q.Get("bookdesk")

	sess, _ := app.currentSession(r)
	bookUser := sess.Samaccountname
	bookFullname := sess.Fullname
	if bookFullname == "" {
		bookFullname = bookUser
	}
	bookPhone := sess.Phone
	bookMail := sess.Mail

	// Cleanup: drop past bookings per map (using each map's local date).
	currentDate := app.db.MapToday(bookMap)
	maps, _ := app.db.ListMaps()
	existing, _ := app.db.ListBookings()
	for _, m := range maps {
		today := app.db.MapToday(m.Mapname)
		if mode == "book" && bookMap == m.Mapname {
			currentDate = today
		}
		for _, b := range existing {
			if b.Map == m.Mapname && b.Date < today {
				_ = app.db.DeleteBooking(b.ID)
			}
		}
	}

	// Refresh after cleanup.
	bookings, _ := app.db.ListBookings()

	status := "error"
	message := "no mode selected"
	data := []map[string]string{}
	debug := map[string]string{}

	switch mode {
	case "book":
		if bookUser != "" && bookDate != "" && bookMap != "" && bookDesk != "" {
			taken := false
			for _, b := range bookings {
				if b.Date == bookDate && b.Map == bookMap && b.Desk == bookDesk {
					taken = true
					break
				}
			}
			switch {
			case taken:
				status, message = "error", "Already booked."
			case bookDate >= currentDate:
				_ = app.db.AddBooking(Booking{
					Date: bookDate, Map: bookMap, Desk: bookDesk,
					User: bookUser, Fullname: bookFullname, Phone: bookPhone, Mail: bookMail,
				})
				status, message = "ok", bookDesk+" booked"
			default:
				status, message = "error", "Date in the past"
			}
		} else {
			status, message = "error", "missing data"
			debug["bookuser"] = bookUser
			debug["bookdate"] = bookDate
			debug["bookmap"] = bookMap
			debug["bookdesk"] = bookDesk
		}

	case "remove":
		if bookUser != "" && bookDate != "" && bookMap != "" && bookDesk != "" {
			for _, b := range bookings {
				if b.User == bookUser && b.Date == bookDate && b.Map == bookMap && b.Desk == bookDesk {
					_ = app.db.DeleteBooking(b.ID)
				}
			}
			status, message = "ok", "Booking cancelled: "+bookDate+" "+bookMap+" "+bookDesk
		} else {
			status, message = "error", "not logged in or no desk provided"
		}

	case "list":
		var filtered []Booking
		switch {
		case bookMap != "":
			for _, b := range bookings {
				if b.Map == bookMap {
					filtered = append(filtered, b)
				}
			}
		case bookUser != "":
			for _, b := range bookings {
				if b.User == bookUser {
					filtered = append(filtered, b)
				}
			}
		default:
			filtered = bookings
			message = "no user or map provided"
		}
		sortBookingsByDate(filtered)
		for _, b := range filtered {
			data = append(data, map[string]string{
				"date": b.Date, "map": b.Map, "desk": b.Desk, "user": b.User,
				"name": b.Fullname, "phone": b.Phone, "mail": b.Mail,
			})
		}
		status = "ok"
		message = itoa(len(data)) + " bookings found"
	}

	writeJSON(w, map[string]interface{}{
		"status":  status,
		"message": message,
		"date":    currentDate,
		"data":    data,
		"debug":   debug,
	})
}

func sortBookingsByDate(b []Booking) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j-1].Date > b[j].Date; j-- {
			b[j-1], b[j] = b[j], b[j-1]
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
