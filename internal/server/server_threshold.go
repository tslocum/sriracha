package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	. "codeberg.org/tslocum/sriracha/model"
	. "codeberg.org/tslocum/sriracha/util"
)

func (s *Server) loadThresholdForm(db serverDB, r *http.Request, t *Threshold) {
	t.Everyone = FormBool(r, "everyone")
	t.Amount = FormInt(r, "amount")
	t.Event = FormRange(r, "event", EventPost, EventReport)
	t.Everywhere = FormBool(r, "board")
	t.Duration = FormInt(r, "duration")
	t.Action = FormString(r, "action")
}

func (s *Server) serveThreshold(data *templateData, db serverDB, w http.ResponseWriter, r *http.Request) {
	if data.forbidden(w, RoleAdmin) {
		return
	}

	var err error
	data.Template = "manage_threshold"
	data.Boards = db.AllBoards()
	data.Manage.Threshold = &Threshold{
		Amount:   1,
		Duration: 30,
	}

	deleteThresholdID := PathInt(r, "/sriracha/threshold/delete/")
	if deleteThresholdID > 0 {
		t := db.ThresholdByID(deleteThresholdID)
		if t == nil {
			data.ManageError("Invalid threshold.")
			return
		}

		db.DeleteThreshold(t.ID)
		s.refreshThresholdCache(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Deleted threshold #%d", t.ID), "")

		data.Redirect(w, r, "/sriracha/threshold/")
		return
	}

	thresholdID, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/sriracha/threshold/"))
	if err == nil && thresholdID > 0 {
		data.Manage.Threshold = db.ThresholdByID(thresholdID)

		if data.Manage.Threshold != nil && r.Method == http.MethodPost {
			s.loadThresholdForm(db, r, data.Manage.Threshold)

			err = data.Manage.Threshold.Validate()
			if err != nil {
				data.ManageError(err.Error())
				return
			}

			// TODO check existing

			db.UpdateThreshold(data.Manage.Threshold)
			s.refreshThresholdCache(db)

			s.log(db, data.Account, nil, fmt.Sprintf("Updated >>/threshold/%d", data.Manage.Threshold.ID), "")

			data.Redirect(w, r, "/sriracha/threshold/")
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		t := &Threshold{}
		s.loadThresholdForm(db, r, t)

		err = t.Validate()
		if err != nil {
			data.ManageError(err.Error())
			return
		}

		// TODO check existing

		db.AddThreshold(t)
		s.refreshThresholdCache(db)

		s.log(db, data.Account, nil, fmt.Sprintf("Added >>/threshold/%d", t.ID), "")

		data.Redirect(w, r, "/sriracha/threshold/")
		return
	}

	data.Manage.Thresholds = db.AllThresholds()
}
