package db

import (
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shyampundkar/entain-master/racing/proto/racing"
)

// RacesRepo provides repository access to races.
type RacesRepo interface {
	// Init will initialise our races repository.
	Init() error

	// List will return a list of races.
	List(filter *racing.ListRacesRequest) ([]*racing.Race, error)
}

type racesRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewRacesRepo creates a new races repository.
func NewRacesRepo(db *sql.DB) RacesRepo {
	return &racesRepo{db: db}
}

// Init prepares the race repository dummy data.
func (r *racesRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy races.
		err = r.seed()
	})

	return err
}

func (r *racesRepo) List(request *racing.ListRacesRequest) ([]*racing.Race, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getRaceQueries()[racesList]

	query, args = r.applyFilter(query, request.Filter)

	query = applyOrderBy(query, request.Orderby)

	log.Printf("Query:%v \n", query)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return r.scanRaces(rows)
}

// Apply sorting & ordering clause to the query
func applyOrderBy(query string, orderBy string) string {
	if len(strings.TrimSpace(orderBy)) != 0 {
		query += " ORDER BY " + orderBy
	}
	return query
}

func (r *racesRepo) applyFilter(query string, filter *racing.ListRacesRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.MeetingIds) > 0 {
		clauses = append(clauses, "meeting_id IN ("+strings.Repeat("?,", len(filter.MeetingIds)-1)+"?)")

		for _, meetingID := range filter.MeetingIds {
			args = append(args, meetingID)
		}
	}

	// Get race visibility filter condition
	raceFilter := getRaceVisibilityFilter(filter.OptionalRaceVisibility)
	if len(raceFilter) != 0 {
		// Keep condition to become part of Where clause later
		clauses = append(clauses, raceFilter)
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}

// Get race visibility filter criteria from the race_visibility
func getRaceVisibilityFilter(race_visibility racing.ListRacesRequestFilter_Visibility) string {
	switch race_visibility {
	case racing.ListRacesRequestFilter_HIDDEN:
		return "visible = false"
	case racing.ListRacesRequestFilter_VISIBLE:
		return "visible = true"
	case racing.ListRacesRequestFilter_SHOW_ALL:
		return ""
	default:
		log.Printf("invalid value for filter.RaceVisibility:%v, Type: %T\n", race_visibility, race_visibility)
	}
	return ""
}

func (m *racesRepo) scanRaces(
	rows *sql.Rows,
) ([]*racing.Race, error) {
	var races []*racing.Race

	for rows.Next() {
		var race racing.Race
		var advertisedStart time.Time

		if err := rows.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}

			return nil, err
		}

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, err
		}

		race.AdvertisedStartTime = ts

		races = append(races, &race)
	}

	return races, nil
}
