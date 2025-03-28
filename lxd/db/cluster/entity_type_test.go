package cluster

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/suite"
)

type selectorSuite struct {
	suite.Suite
	db *sql.DB
}

func TestSelectorSuite(t *testing.T) {
	suite.Run(t, new(selectorSuite))
}

func (s *selectorSuite) SetupTest() {
	s.db = newDB(s.T())
}

func (s *selectorSuite) Test_entityTypeClusterGroup_selectorQuery() {
	tests := []struct {
		name          string
		selector      Selector
		expectedQuery string
		expectedArgs  []any
		expectedErr   error
	}{
		{
			selector: Selector{
				ID:         0,
				EntityType: EntityType("cluster_group"),
				Matchers: SelectorMatchers{
					{
						Property: "name",
						Values:   []string{"g1", "g2"},
					},
				},
			},
			expectedQuery: `SELECT id FROM cluster_groups WHERE name IN (?, ?)`,
			expectedArgs:  []any{"g1", "g2"},
		},
	}

	for _, tt := range tests {
		query, args, err := entityTypeClusterGroup{}.selectorQuery(tt.selector)
		if tt.expectedErr != nil {
			s.Equal(tt.expectedErr, err)
			return
		}

		s.Require().NoError(err)

		_, err = s.db.Prepare(query)
		s.Require().NoError(err)

		s.Equal(tt.expectedQuery, query)
		s.ElementsMatch(tt.expectedArgs, args)
	}
}
