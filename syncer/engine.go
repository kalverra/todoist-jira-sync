package syncer

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	jiraCloud "github.com/andygrunwald/go-jira/v2/cloud"
	"github.com/ankitpokhrel/jira-cli/pkg/md"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/kalverra/todoist-jira-sync/config"
	"github.com/kalverra/todoist-jira-sync/jira"
	"github.com/kalverra/todoist-jira-sync/todoist"
)

const (
	commentFromJiraPrefix = "[From Jira %s] " // %s is the Jira issue key
	defaultIssueType      = "Story"
	linkLabel             = "jira-sync"
)

// Engine orchestrates bidirectional sync between Todoist and Jira.
type Engine struct {
	todoist *todoist.Client
	jira    *jira.Client
	cfg     *config.Config
	logger  zerolog.Logger
}

// NewEngine creates a new sync engine.
func NewEngine(
	todoistClient *todoist.Client,
	jiraClient *jira.Client,
	cfg *config.Config,
	logger zerolog.Logger,
) *Engine {
	return &Engine{
		todoist: todoistClient,
		jira:    jiraClient,
		cfg:     cfg,
		logger:  logger.With().Str("component", "syncer").Logger(),
	}
}

type sectionMap struct {
	byID   map[string]string
	byName map[string]string
}

type syncAction struct {
	jiraKey string
	summary string
}

type syncSummary struct {
	createdJira      []syncAction
	createdTodoist   []syncAction
	updatedToTodoist []syncAction
	updatedToJira    []syncAction
	completedTodoist []syncAction
	resolvedJira     []syncAction
	errors           []syncAction
}

func (s *syncSummary) print(duration time.Duration) {
	var b strings.Builder
	b.WriteString("\n================================\n")
	b.WriteString("  Sync Summary\n")
	b.WriteString("================================\n")

	sections := []struct {
		label   string
		actions []syncAction
	}{
		{"Created in Jira", s.createdJira},
		{"Created in Todoist", s.createdTodoist},
		{"Updated Jira -> Todoist", s.updatedToTodoist},
		{"Updated Todoist -> Jira", s.updatedToJira},
		{"Completed in Todoist", s.completedTodoist},
		{"Resolved in Jira", s.resolvedJira},
		{"Errors", s.errors},
	}

	anyActivity := false
	for _, sec := range sections {
		if len(sec.actions) == 0 {
			continue
		}
		anyActivity = true
		fmt.Fprintf(&b, "\n%s (%d):\n", sec.label, len(sec.actions))
		for _, a := range sec.actions {
			if a.jiraKey != "" {
				fmt.Fprintf(&b, "  - [%s] %s\n", a.jiraKey, a.summary)
			} else {
				fmt.Fprintf(&b, "  - %s\n", a.summary)
			}
		}
	}

	if !anyActivity {
		b.WriteString("\nEverything is up to date.\n")
	}

	fmt.Fprintf(&b, "\nCompleted in %s\n", duration.Truncate(time.Millisecond))
	b.WriteString("================================\n")
	fmt.Print(b.String())
}

// Run executes a single sync cycle.
func (e *Engine) Run(ctx context.Context) error {
	start := time.Now()
	e.logger.Info().Msg("syncing todoist and jira")

	var (
		project              *todoist.Project
		sections             []todoist.Section
		secMap               sectionMap
		tasks                []todoist.Task
		completedTodoistKeys map[string]bool
		issues               []jiraCloud.Issue
		eg                   = errgroup.Group{}
		summary              syncSummary
	)

	eg.Go(func() error {
		var todoistErr error
		project, todoistErr = e.todoist.FindProjectByName(ctx, e.cfg.TodoistProject)
		if todoistErr != nil {
			return fmt.Errorf("find todoist project: %w", todoistErr)
		}
		e.logger.Debug().
			Str("project_id", project.ID).
			Str("project_name", project.Name).
			Msg("found todoist project")

		sections, todoistErr = e.todoist.GetSections(ctx, project.ID)
		if todoistErr != nil {
			return fmt.Errorf("get todoist sections: %w", todoistErr)
		}
		secMap = buildSectionMap(sections)

		tasks, todoistErr = e.todoist.GetTasks(ctx, project.ID)
		if todoistErr != nil {
			return fmt.Errorf("get todoist tasks: %w", todoistErr)
		}

		// Fetch recently completed Todoist tasks to detect Todoist→Jira completions.
		since := time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339)
		until := time.Now().UTC().Format(time.RFC3339)
		completedTasks, err := e.todoist.GetCompletedTasks(ctx, project.ID, since, until)
		if err != nil {
			e.logger.Warn().Err(err).Msg("failed to fetch completed todoist tasks, skipping completion sync")
		} else {
			for _, ct := range completedTasks {
				if key := ExtractJiraKey(ct.Content); key != "" {
					completedTodoistKeys[key] = true
				}
			}
		}

		e.logger.Debug().Int("count", len(tasks)).Msg("fetched todoist tasks")
		return nil
	})

	eg.Go(func() error {
		var jiraErr error
		jql := "project = " + e.cfg.JiraProject +
			" AND assignee = currentUser()"
		if typesJQL := e.cfg.JiraIssueTypesJQL(); typesJQL != "" {
			jql += " AND " + typesJQL
		}
		jql += " ORDER BY updated DESC"
		issues, _, jiraErr = e.jira.Issue.SearchV2JQL(ctx, jql, &jiraCloud.SearchOptionsV2{
			MaxResults: 200,
			Fields: []string{
				"key",
				"summary",
				"description",
				"status",
				"updated",
				"duedate",
				"comments",
				"priority",
				"resolution",
				"sprint",
			},
		})
		if jiraErr != nil {
			return fmt.Errorf("search jira issues: %w", jiraErr)
		}
		e.logger.Debug().Int("count", len(issues)).Msg("fetched jira issues")
		return nil
	})

	err := eg.Wait()
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// Classify Todoist tasks:
	// - Has Jira key prefix in content -> linked
	// - Has jira-sync label but no key -> unlinked, needs Jira issue.
	// - No label -> ignored
	todoistByJiraKey := make(map[string]*todoist.Task)
	var unlinkedTodoistTasks []*todoist.Task
	for i := range tasks {
		jiraKey := ExtractJiraKey(tasks[i].Content)
		if jiraKey != "" {
			todoistByJiraKey[jiraKey] = &tasks[i]
		} else if slices.Contains(tasks[i].Labels, linkLabel) {
			unlinkedTodoistTasks = append(unlinkedTodoistTasks, &tasks[i])
		}
	}

	// Classify Jira issues:
	// - Linked to an active Todoist task -> sync pair
	// - Not linked but matching a recently completed Todoist task -> resolve Jira
	// - Already resolved -> skip
	// - Otherwise -> new, create Todoist task
	var unlinkedJiraIssues []*jiraCloud.Issue
	for i := range issues {
		if _, linked := todoistByJiraKey[issues[i].Key]; linked {
			continue
		}
		if completedTodoistKeys[issues[i].Key] {
			e.resolveJiraIssue(ctx, &issues[i], &summary)
			continue
		}
		if issues[i].Fields.Resolution != nil {
			continue
		}
		if issues[i].Fields.Sprint == nil || issues[i].Fields.Sprint.State != "active" {
			e.logger.Debug().
				Str("issue_key", issues[i].Key).
				Msg("jira issue not in active sprint, skipping todoist creation")
			continue
		}
		unlinkedJiraIssues = append(unlinkedJiraIssues, &issues[i])
	}

	for _, task := range unlinkedTodoistTasks {
		if err := e.createJiraFromTodoist(ctx, task, secMap, &summary); err != nil {
			e.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("task", task.Content).
				Msg("failed to create jira issue from todoist task")
			summary.errors = append(summary.errors, syncAction{summary: "create Jira from: " + task.Content})
		}
	}

	for _, issue := range unlinkedJiraIssues {
		if err := e.createTodoistFromJira(ctx, issue, project.ID, secMap, &summary); err != nil {
			e.logger.Error().Err(err).
				Str("issue_key", issue.Key).
				Str("summary", issue.Fields.Summary).
				Msg("failed to create todoist task from jira issue")
			summary.errors = append(
				summary.errors,
				syncAction{jiraKey: issue.Key, summary: "create Todoist from: " + issue.Fields.Summary},
			)
		}
	}

	for jiraKey, task := range todoistByJiraKey {
		issue, ok := findIssueByKey(issues, jiraKey)
		if !ok {
			e.logger.Warn().
				Str("jira_key", jiraKey).
				Str("task_id", task.ID).
				Str("task", task.Content).
				Msg("linked jira issue not found, skipping")
			continue
		}
		if err := e.syncLinkedPair(ctx, task, issue, project.ID, secMap, &summary); err != nil {
			e.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("issue_key", issue.Key).
				Str("issue", issue.Fields.Summary).
				Msg("failed to sync linked pair")
			summary.errors = append(
				summary.errors,
				syncAction{jiraKey: issue.Key, summary: "sync: " + issue.Fields.Summary},
			)
		}
	}

	elapsed := time.Since(start)
	e.logger.Info().
		Str("duration", elapsed.String()).
		Msg("sync complete")

	summary.print(elapsed)
	return nil
}

func (e *Engine) createJiraFromTodoist(
	ctx context.Context,
	task *todoist.Task,
	secMap sectionMap,
	s *syncSummary,
) error {
	if !slices.Contains(task.Labels, linkLabel) {
		return nil
	}
	sectionName := secMap.byID[task.SectionID]
	jiraStatus := e.cfg.JiraStatusForSection(sectionName)

	created, _, err := e.jira.Issue.Create(ctx, &jiraCloud.Issue{
		Fields: &jiraCloud.IssueFields{
			Project:     jiraCloud.Project{Key: e.cfg.JiraProject},
			Summary:     task.Content,
			Description: md.ToJiraMD(task.Description),
			Type:        jiraCloud.IssueType{Name: defaultIssueType},
		},
	})
	if err != nil {
		return fmt.Errorf("create jira issue: %w", err)
	}
	s.createdJira = append(s.createdJira, syncAction{jiraKey: created.Key, summary: task.Content})
	e.logger.Info().
		Str("task_id", task.ID).
		Str("task", task.Content).
		Str("issue_key", created.Key).
		Msg("created jira issue from todoist task")

	linkedContent := PrependJiraLink(task.Content, created.Key, e.cfg.JiraURL)
	_, err = e.todoist.UpdateTask(ctx, task.ID, todoist.UpdateTaskRequest{
		Content: &linkedContent,
	})
	if err != nil {
		return fmt.Errorf("update todoist task content with jira link: %w", err)
	}

	if jiraStatus != "" && jiraStatus != "To Do" {
		if _, err := e.jira.Issue.DoTransition(ctx, created.Key, jiraStatus); err != nil {
			e.logger.Warn().Err(err).
				Str("issue_key", created.Key).
				Str("target_status", jiraStatus).
				Str("issue", created.Fields.Summary).
				Msg("failed to transition new issue")
		}
	}

	return nil
}

func (e *Engine) createTodoistFromJira(
	ctx context.Context,
	issue *jiraCloud.Issue,
	projectID string,
	secMap sectionMap,
	s *syncSummary,
) error {
	statusName := ""
	if issue.Fields.Status != nil {
		statusName = issue.Fields.Status.Name
	}
	sectionName := e.cfg.SectionForJiraStatus(statusName)
	sectionID := secMap.byName[sectionName]

	if sectionID == "" && sectionName != "" {
		sec, err := e.todoist.CreateSection(ctx, projectID, sectionName)
		if err != nil {
			return fmt.Errorf("create todoist section %q: %w", sectionName, err)
		}
		sectionID = sec.ID
		secMap.byID[sec.ID] = sectionName
		secMap.byName[sectionName] = sec.ID
	}

	linkedContent := PrependJiraLink(issue.Fields.Summary, issue.Key, e.cfg.JiraURL)
	createReq := todoist.CreateTaskRequest{
		Content:     linkedContent,
		Description: md.FromJiraMD(issue.Fields.Description),
		ProjectID:   projectID,
		SectionID:   sectionID,
		Labels:      []string{linkLabel},
	}
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		createReq.DueDate = duedate.Format("2006-01-02")
	}

	task, err := e.todoist.CreateTask(ctx, createReq)
	if err != nil {
		return fmt.Errorf("create todoist task: %w", err)
	}
	s.createdTodoist = append(s.createdTodoist, syncAction{jiraKey: issue.Key, summary: issue.Fields.Summary})
	e.logger.Info().
		Str("issue_key", issue.Key).
		Str("task_id", task.ID).
		Str("task", task.Content).
		Str("issue", issue.Fields.Summary).
		Msg("created todoist task from jira issue")

	if err := e.syncCommentsToTodoist(ctx, issue, task.ID); err != nil {
		e.logger.Warn().Err(err).
			Str("task_id", task.ID).
			Str("task", task.Content).
			Str("issue_key", issue.Key).
			Str("issue", issue.Fields.Summary).
			Msg("failed to sync comments to new todoist task")
	}

	return nil
}

func (e *Engine) syncLinkedPair(
	ctx context.Context,
	task *todoist.Task,
	issue *jiraCloud.Issue,
	projectID string,
	secMap sectionMap,
	s *syncSummary,
) error {
	if issue.Fields.Resolution != nil {
		e.logger.Info().
			Str("task_id", task.ID).
			Str("issue_key", issue.Key).
			Msg("jira issue resolved, closing todoist task")
		s.completedTodoist = append(s.completedTodoist, syncAction{jiraKey: issue.Key, summary: issue.Fields.Summary})
		return e.todoist.CloseTask(ctx, task.ID)
	}

	jiraUpdated := time.Time(issue.Fields.Updated)

	todoistUpdated, err := time.Parse(time.RFC3339Nano, task.UpdatedAt)
	if err != nil {
		e.logger.Warn().Err(err).
			Str("task_id", task.ID).
			Msg("could not parse todoist updated_at, assuming todoist is newer")
		s.updatedToJira = append(s.updatedToJira, syncAction{jiraKey: issue.Key, summary: issue.Fields.Summary})
		return e.pushTodoistToJira(ctx, task, issue, secMap)
	}

	if jiraUpdated.After(todoistUpdated) {
		e.logger.Debug().
			Str("task_id", task.ID).
			Str("issue_key", issue.Key).
			Msg("jira is newer, syncing jira -> todoist")
		s.updatedToTodoist = append(s.updatedToTodoist, syncAction{jiraKey: issue.Key, summary: issue.Fields.Summary})
		return e.pushJiraToTodoist(ctx, task, issue, projectID, secMap)
	}

	e.logger.Debug().
		Str("task_id", task.ID).
		Str("issue_key", issue.Key).
		Msg("todoist is newer (or same), syncing todoist -> jira")
	s.updatedToJira = append(s.updatedToJira, syncAction{jiraKey: issue.Key, summary: issue.Fields.Summary})
	return e.pushTodoistToJira(ctx, task, issue, secMap)
}

func (e *Engine) pushJiraToTodoist(
	ctx context.Context,
	task *todoist.Task,
	issue *jiraCloud.Issue,
	projectID string,
	secMap sectionMap,
) error {
	linkedContent := PrependJiraLink(issue.Fields.Summary, issue.Key, e.cfg.JiraURL)
	desc := md.FromJiraMD(issue.Fields.Description)

	updateReq := todoist.UpdateTaskRequest{
		Content:     &linkedContent,
		Description: &desc,
	}
	duedate := time.Time(issue.Fields.Duedate)
	if !duedate.IsZero() {
		duedateStr := duedate.Format("2006-01-02")
		updateReq.DueDate = &duedateStr
	}

	if _, err := e.todoist.UpdateTask(ctx, task.ID, updateReq); err != nil {
		return fmt.Errorf("update todoist task: %w", err)
	}

	if issue.Fields.Status != nil {
		targetSection := e.cfg.SectionForJiraStatus(issue.Fields.Status.Name)
		currentSection := secMap.byID[task.SectionID]
		if targetSection != currentSection {
			targetSectionID := secMap.byName[targetSection]
			if targetSectionID == "" {
				sec, err := e.todoist.CreateSection(ctx, projectID, targetSection)
				if err != nil {
					return fmt.Errorf("create todoist section %q: %w", targetSection, err)
				}
				targetSectionID = sec.ID
				secMap.byID[sec.ID] = targetSection
				secMap.byName[targetSection] = sec.ID
			}
			if err := e.todoist.MoveTaskToSection(ctx, task.ID, targetSectionID); err != nil {
				return fmt.Errorf("move todoist task to section: %w", err)
			}
		}
	}

	if err := e.syncCommentsToTodoist(ctx, issue, task.ID); err != nil {
		e.logger.Warn().Err(err).
			Str("task_id", task.ID).
			Str("task", task.Content).
			Str("issue_key", issue.Key).
			Str("issue", issue.Fields.Summary).
			Msg("failed to sync comments jira -> todoist")
	}

	return nil
}

func (e *Engine) pushTodoistToJira(
	ctx context.Context,
	task *todoist.Task,
	issue *jiraCloud.Issue,
	secMap sectionMap,
) error {
	summary := StripJiraPrefix(task.Content)

	updateIssue := &jiraCloud.Issue{
		Key: issue.Key,
		Fields: &jiraCloud.IssueFields{
			Summary:     summary,
			Description: md.ToJiraMD(task.Description),
		},
	}
	if task.Due != nil && task.Due.Date != "" {
		dueTime, err := time.Parse("2006-01-02", task.Due.Date)
		if err == nil {
			updateIssue.Fields.Duedate = jiraCloud.Date(dueTime)
		}
	}

	if _, _, err := e.jira.Issue.Update(ctx, updateIssue, nil); err != nil {
		return fmt.Errorf("update jira issue: %w", err)
	}

	sectionName := secMap.byID[task.SectionID]
	if sectionName != "" {
		targetJiraStatus := e.cfg.JiraStatusForSection(sectionName)
		currentStatus := ""
		if issue.Fields.Status != nil {
			currentStatus = issue.Fields.Status.Name
		}
		if !strings.EqualFold(targetJiraStatus, currentStatus) {
			if _, err := e.jira.Issue.DoTransition(ctx, issue.Key, targetJiraStatus); err != nil {
				e.logger.Warn().Err(err).
					Str("issue_key", issue.Key).
					Str("target", targetJiraStatus).
					Str("issue_key", issue.Key).
					Str("issue", issue.Fields.Summary).
					Msg("failed to transition jira issue")
			}
		}
	}

	return nil
}

// syncCommentsToTodoist syncs comments from a Jira issue to a Todoist task.
// It uses the issue's embedded Comments field to avoid a separate API call.
func (e *Engine) syncCommentsToTodoist(
	ctx context.Context,
	issue *jiraCloud.Issue,
	todoistTaskID string,
) error {
	if issue.Fields.Comments == nil {
		return nil
	}

	todoistComments, err := e.todoist.GetComments(ctx, todoistTaskID)
	if err != nil {
		return fmt.Errorf("get todoist comments: %w", err)
	}

	existingComments := make([]string, 0, len(todoistComments))
	for _, c := range todoistComments {
		existingComments = append(existingComments, c.Content)
	}

	for _, jc := range issue.Fields.Comments.Comments {
		syncedContent := fmt.Sprintf(commentFromJiraPrefix, jc.Author.Name) + md.FromJiraMD(jc.Body)
		if slices.Contains(existingComments, syncedContent) {
			continue
		}
		_, err := e.todoist.CreateComment(ctx, todoist.CreateCommentRequest{
			TaskID:  todoistTaskID,
			Content: syncedContent,
		})
		if err != nil {
			e.logger.Error().Err(err).
				Str("task_id", todoistTaskID).
				Msg("failed to add comment to todoist")
		}
	}

	return nil
}

// resolveJiraIssue transitions a Jira issue to "Done". Called when a
// linked Todoist task was recently completed.
func (e *Engine) resolveJiraIssue(ctx context.Context, issue *jiraCloud.Issue, s *syncSummary) {
	if issue.Fields.Resolution != nil {
		e.logger.Debug().
			Str("issue_key", issue.Key).
			Msg("jira issue already resolved, skipping")
		return
	}

	e.logger.Info().
		Str("issue_key", issue.Key).
		Str("summary", issue.Fields.Summary).
		Msg("todoist task completed, resolving jira issue")

	if _, err := e.jira.Issue.DoTransition(ctx, issue.Key, "Done"); err != nil {
		e.logger.Error().Err(err).
			Str("issue_key", issue.Key).
			Msg("failed to transition jira issue to Done")
		s.errors = append(s.errors, syncAction{jiraKey: issue.Key, summary: "resolve: " + issue.Fields.Summary})
		return
	}
	s.resolvedJira = append(s.resolvedJira, syncAction{jiraKey: issue.Key, summary: issue.Fields.Summary})
}

func buildSectionMap(sections []todoist.Section) sectionMap {
	sm := sectionMap{
		byID:   make(map[string]string, len(sections)),
		byName: make(map[string]string, len(sections)),
	}
	for _, s := range sections {
		sm.byID[s.ID] = s.Name
		sm.byName[s.Name] = s.ID
	}
	return sm
}

func findIssueByKey(issues []jiraCloud.Issue, key string) (*jiraCloud.Issue, bool) {
	for i := range issues {
		if issues[i].Key == key {
			return &issues[i], true
		}
	}
	return nil, false
}
