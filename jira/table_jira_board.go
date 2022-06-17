package jira

import (
	"context"

	"github.com/andygrunwald/go-jira"
	"github.com/turbot/steampipe-plugin-sdk/v3/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v3/plugin/transform"

	"github.com/turbot/steampipe-plugin-sdk/v3/plugin"
)

//// TABLE DEFINITION

func tableBoard(_ context.Context) *plugin.Table {
	return &plugin.Table{
		Name:        "jira_board",
		Description: "A board displays issues from one or more projects, giving you a flexible way of viewing, managing, and reporting on work in progress.",
		Get: &plugin.GetConfig{
			KeyColumns: plugin.SingleColumn("id"),
			Hydrate:    getBoard,
		},
		List: &plugin.ListConfig{
			Hydrate: listBoards,
		},
		Columns: []*plugin.Column{
			{
				Name:        "id",
				Description: "The ID of the board.",
				Type:        proto.ColumnType_INT,
				Transform:   transform.FromGo(),
			},
			{
				Name:        "name",
				Description: "The name of the board.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "self",
				Description: "The URL of the board details.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "type",
				Description: "The board type of the board. Valid values are simple, scrum and kanban.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "filter_id",
				Description: "Filter id of the board.",
				Type:        proto.ColumnType_INT,
				Hydrate:     getBoardConfiguration,
				Transform:   transform.FromField("Filter.ID"),
			},
			{
				Name:        "sub_query",
				Description: "JQL subquery used by the given board - (Kanban only).",
				Type:        proto.ColumnType_STRING,
				Hydrate:     getBoardConfiguration,
				Transform:   transform.FromField("SubQuery.Query"),
			},

			// Standard columns
			{
				Name:        "title",
				Description: ColumnDescriptionTitle,
				Type:        proto.ColumnType_STRING,
				Transform:   transform.FromField("Name"),
			},
		},
	}
}

//// LIST FUNCTION

func listBoards(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {
	client, err := connect(ctx, d)
	if err != nil {
		plugin.Logger(ctx).Error("jira_board.listBoards", "connection_error", err)
		return nil, err
	}

	last := 0
	// If the requested number of items is less than the paging max limit
	// set the limit to that instead
	queryLimit := d.QueryContext.Limit
	var maxResults int = 1000
	if d.QueryContext.Limit != nil {
		if *queryLimit < 1000 {
			maxResults = int(*queryLimit)
		}
	}
	for {
		opt := jira.SearchOptions{
			MaxResults: maxResults,
			StartAt:    last,
		}

		boardList, resp, err := client.Board.GetAllBoardsWithContext(ctx, &jira.BoardListOptions{
			SearchOptions: opt,
		})
		if err != nil {
			defer resp.Body.Close()
			plugin.Logger(ctx).Error("jira_board.listBoards", "api_error", err)
			plugin.Logger(ctx).Debug("jira_project.listProjects", "response", resp.Body)
			return nil, err
		}

		total := resp.Total

		for _, board := range boardList.Values {
			d.StreamListItem(ctx, board)
			// Context may get cancelled due to manual cancellation or if the limit has been reached
			if d.QueryStatus.RowsRemaining(ctx) == 0 {
				return nil, nil
			}
		}

		last = resp.StartAt + len(boardList.Values)
		if last >= total {
			return nil, nil
		}
	}
}

//// HYDRATE FUNCTIONS

func getBoard(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {
	boardId := d.KeyColumnQuals["id"].GetInt64Value()
	if boardId == 0 {
		return nil, nil
	}
	client, err := connect(ctx, d)
	if err != nil {
		plugin.Logger(ctx).Error("jira_board.getBoard", "connection_error", err)
		return nil, err
	}

	board, res, err := client.Board.GetBoard(int(boardId))
	if err != nil {
		defer res.Body.Close()
		if isNotFoundError(err) {
			return nil, nil
		}
		plugin.Logger(ctx).Error("jira_board.getBoard", "api_error", err)
		plugin.Logger(ctx).Debug("jira_board.getBoard", "response", res.Body)
		return nil, err
	}

	return *board, err
}

func getBoardConfiguration(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	board := h.Item.(jira.Board)

	client, err := connect(ctx, d)
	if err != nil {
		plugin.Logger(ctx).Error("jira_board.getBoardConfiguration", "connection_error", err)
		return nil, err
	}

	boardConfiguration, res, err := client.Board.GetBoardConfiguration(board.ID)
	if err != nil {
		defer res.Body.Close()
		plugin.Logger(ctx).Error("jira_board.getBoardConfiguration", "api_error", err)
		plugin.Logger(ctx).Debug("jira_board.getBoardConfiguration", "response", res.Body)
		return nil, err
	}

	return boardConfiguration, err
}
