package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CrawledPage struct {
	URL   string `json:"url" bson:"url"`
	Title string `json:"title" bson:"title"`
}

type Question struct {
	Question    string   `json:"question" bson:"question"`
	Relevance   string   `json:"relevance" bson:"relevance"`
	Category    string   `json:"category" bson:"category"`
	PageURLs    []string `json:"page_urls" bson:"pageUrls"`
	BrandStatus string   `json:"brand_status,omitempty" bson:"brandStatus,omitempty"`
}

type AnalysisResult struct {
	SiteSummary  string        `json:"site_summary" bson:"siteSummary"`
	Questions    []Question    `json:"questions" bson:"questions"`
	CrawledPages []CrawledPage `json:"crawled_pages" bson:"crawledPages"`
}

type Analysis struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID  string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain    string             `json:"domain" bson:"domain"`
	RawText   string             `json:"raw_text" bson:"rawText"`
	Result                AnalysisResult     `json:"result" bson:"result"`
	Model                 string             `json:"model" bson:"model"`
	BrandContextUsed      bool               `json:"brand_context_used" bson:"brandContextUsed"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty" bson:"brandProfileUpdatedAt,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// Answer optimization types

type DimensionScore struct {
	Score        int      `json:"score" bson:"score"`
	Evidence     []string `json:"evidence" bson:"evidence"`
	Improvements []string `json:"improvements" bson:"improvements"`
}

type Competitor struct {
	Domain        string `json:"domain" bson:"domain"`
	ScoreEstimate int    `json:"score_estimate" bson:"scoreEstimate"`
	Strengths     string `json:"strengths" bson:"strengths"`
}

type Recommendation struct {
	Priority       string `json:"priority" bson:"priority"`
	Action         string `json:"action" bson:"action"`
	ExpectedImpact string `json:"expected_impact" bson:"expectedImpact"`
	Dimension      string `json:"dimension" bson:"dimension"`
}

type OptimizationResult struct {
	OverallScore           int              `json:"overall_score" bson:"overallScore"`
	ContentAuthority       DimensionScore   `json:"content_authority" bson:"contentAuthority"`
	StructuralOptimization DimensionScore   `json:"structural_optimization" bson:"structuralOptimization"`
	SourceAuthority        DimensionScore   `json:"source_authority" bson:"sourceAuthority"`
	KnowledgePersistence   DimensionScore   `json:"knowledge_persistence" bson:"knowledgePersistence"`
	Competitors            []Competitor     `json:"competitors" bson:"competitors"`
	Recommendations        []Recommendation `json:"recommendations" bson:"recommendations"`
}

type Optimization struct {
	ID            primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID      string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	AnalysisID    primitive.ObjectID `json:"analysis_id" bson:"analysisId"`
	QuestionIndex int                `json:"question_index" bson:"questionIndex"`
	Question      string             `json:"question" bson:"question"`
	Domain        string             `json:"domain" bson:"domain"`
	PageURLs      []string           `json:"page_urls" bson:"pageUrls"`
	Result        OptimizationResult `json:"result" bson:"result"`
	RawText       string             `json:"raw_text" bson:"rawText"`
	BrandStatus   string             `json:"brand_status,omitempty" bson:"brandStatus,omitempty"`
	Model                 string             `json:"model" bson:"model"`
	Public                bool               `json:"public" bson:"public"`
	BrandContextUsed      bool               `json:"brand_context_used" bson:"brandContextUsed"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty" bson:"brandProfileUpdatedAt,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// Todo item types

type TodoItem struct {
	ID              primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	TenantID        string              `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	OptimizationID  primitive.ObjectID  `json:"optimization_id" bson:"optimizationId"`
	AnalysisID      primitive.ObjectID  `json:"analysis_id" bson:"analysisId"`
	VideoAnalysisID *primitive.ObjectID `json:"video_analysis_id,omitempty" bson:"videoAnalysisId,omitempty"`
	SourceType      string              `json:"source_type" bson:"sourceType"` // "optimization" or "video"
	Domain          string              `json:"domain" bson:"domain"`
	Question        string              `json:"question" bson:"question"`
	Action          string              `json:"action" bson:"action"`
	Summary         string              `json:"summary" bson:"summary"`
	ExpectedImpact  string              `json:"expected_impact" bson:"expectedImpact"`
	Dimension       string              `json:"dimension" bson:"dimension"`
	Priority        string              `json:"priority" bson:"priority"`
	Status          string              `json:"status" bson:"status"` // "todo", "completed", "backlogged", "archived"
	CreatedAt       time.Time           `json:"created_at" bson:"createdAt"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty" bson:"completedAt,omitempty"`
	BackloggedAt    *time.Time          `json:"backlogged_at,omitempty" bson:"backloggedAt,omitempty"`
	ArchivedAt      *time.Time          `json:"archived_at,omitempty" bson:"archivedAt,omitempty"`
	Tags            []string            `json:"tags,omitempty" bson:"tags,omitempty"`
}

// OptimizationSummary is the lightweight shape returned by the optimizations list endpoint.
type OptimizationSummary struct {
	ID            primitive.ObjectID `json:"id" bson:"_id"`
	Domain        string             `json:"domain" bson:"domain"`
	Question      string             `json:"question" bson:"question"`
	QuestionIndex int                `json:"question_index"`
	OverallScore  int                `json:"overall_score"`
	Model         string             `json:"model" bson:"model"`
	Public                bool               `json:"public" bson:"public"`
	BrandStatus           string             `json:"brand_status,omitempty"`
	BrandContextUsed      bool               `json:"brand_context_used"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// AnalysisSummary is the lightweight shape returned by the list endpoint.
type AnalysisSummary struct {
	ID            primitive.ObjectID `json:"id" bson:"_id"`
	Domain        string             `json:"domain" bson:"domain"`
	SiteSummary   string             `json:"site_summary"`
	QuestionCount int                `json:"question_count"`
	PageCount     int                `json:"page_count"`
	Model                 string             `json:"model"`
	BrandContextUsed      bool               `json:"brand_context_used"`
	BrandProfileUpdatedAt *time.Time         `json:"brand_profile_updated_at,omitempty"`
	CreatedAt             time.Time          `json:"created_at" bson:"createdAt"`
}

// Brand Intelligence types

type BrandCompetitor struct {
	Name         string `json:"name" bson:"name"`
	URL          string `json:"url" bson:"url"`
	Relationship string `json:"relationship" bson:"relationship"` // direct, indirect, aspirational, adjacent
	Notes        string `json:"notes" bson:"notes"`
}

type TargetQuery struct {
	Query    string `json:"query" bson:"query"`
	Priority string `json:"priority" bson:"priority"` // high, medium, low
	Type     string `json:"type" bson:"type"`         // brand, category, comparison, problem
}

type KeyMessage struct {
	Claim       string `json:"claim" bson:"claim"`
	EvidenceURL string `json:"evidence_url" bson:"evidenceUrl"`
	Priority    string `json:"priority" bson:"priority"` // high, medium, low
}

type BrandPresence struct {
	YouTubeURL     string   `json:"youtube_url" bson:"youtubeUrl"`
	Subreddits     []string `json:"subreddits" bson:"subreddits"`
	ReviewSiteURLs []string `json:"review_site_urls" bson:"reviewSiteUrls"`
	SocialProfiles []string `json:"social_profiles" bson:"socialProfiles"`
	ContentAssets  []string `json:"content_assets" bson:"contentAssets"`
	Podcasts       []string `json:"podcasts" bson:"podcasts"`
}

type BrandProfile struct {
	ID              primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID        string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain          string             `json:"domain" bson:"domain"`
	BrandName       string             `json:"brand_name" bson:"brandName"`
	Description     string             `json:"description" bson:"description"`
	Categories      []string           `json:"categories" bson:"categories"`
	Products        []string           `json:"products" bson:"products"`
	PrimaryAudience string             `json:"primary_audience" bson:"primaryAudience"`
	KeyUseCases     []string           `json:"key_use_cases" bson:"keyUseCases"`
	Competitors     []BrandCompetitor  `json:"competitors" bson:"competitors"`
	TargetQueries   []TargetQuery      `json:"target_queries" bson:"targetQueries"`
	KeyMessages     []KeyMessage       `json:"key_messages" bson:"keyMessages"`
	Differentiators []string           `json:"differentiators" bson:"differentiators"`
	Presence         BrandPresence      `json:"presence" bson:"presence"`
	PresenceComplete bool               `json:"presence_complete" bson:"presenceComplete"`
	Public           bool               `json:"public" bson:"public"`
	LastDiscoveryAt  *time.Time         `json:"last_discovery_at,omitempty" bson:"lastDiscoveryAt,omitempty"`
	CreatedAt             time.Time  `json:"created_at" bson:"createdAt"`
	UpdatedAt             time.Time  `json:"updated_at" bson:"updatedAt"`
	BrandContentUpdatedAt *time.Time `json:"brand_content_updated_at,omitempty" bson:"brandContentUpdatedAt,omitempty"`

	VideoSearchAdditions []string `json:"video_search_additions,omitempty" bson:"videoSearchAdditions,omitempty"`
	VideoSearchRemovals  []string `json:"video_search_removals,omitempty" bson:"videoSearchRemovals,omitempty"`
}

type BrandProfileSummary struct {
	ID                    primitive.ObjectID `json:"id" bson:"_id"`
	Domain                string             `json:"domain" bson:"domain"`
	BrandName             string             `json:"brand_name" bson:"brandName"`
	CompetitorCount       int                `json:"competitor_count"`
	QueryCount            int                `json:"query_count"`
	Completeness          int                `json:"completeness"`
	Public                bool               `json:"public" bson:"public"`
	UpdatedAt             time.Time          `json:"updated_at" bson:"updatedAt"`
	BrandContentUpdatedAt *time.Time         `json:"brand_content_updated_at,omitempty" bson:"brandContentUpdatedAt,omitempty"`
}

// Health check persistence

type ModelStatusRecord struct {
	Model      string `json:"model" bson:"model"`
	Name       string `json:"name" bson:"name"`
	Status     string `json:"status" bson:"status"` // "available", "overloaded", "error"
	LatencyMs  int64  `json:"latency_ms,omitempty" bson:"latencyMs,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty" bson:"httpStatus,omitempty"`
}

type HealthRecord struct {
	ID        primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	Models    []ModelStatusRecord `json:"models" bson:"models"`
	CheckedAt time.Time           `json:"checked_at" bson:"checkedAt"`
}

// Domain Summary types

type SummaryTheme struct {
	Title       string   `json:"title" bson:"title"`
	Description string   `json:"description" bson:"description"`
	ReportRefs  []string `json:"report_refs" bson:"reportRefs"`
}

type SummaryActionItem struct {
	Priority       string   `json:"priority" bson:"priority"`
	Action         string   `json:"action" bson:"action"`
	Dimension      string   `json:"dimension" bson:"dimension"`
	ExpectedImpact string   `json:"expected_impact" bson:"expectedImpact"`
	SourceReports  []string `json:"source_reports" bson:"sourceReports"`
}

type SummaryContradiction struct {
	Topic          string   `json:"topic" bson:"topic"`
	Positions      []string `json:"positions" bson:"positions"`
	ReportRefs     []string `json:"report_refs" bson:"reportRefs"`
	Recommendation string   `json:"recommendation" bson:"recommendation"`
}

type DomainSummaryResult struct {
	ExecutiveSummary string                 `json:"executive_summary" bson:"executiveSummary"`
	AverageScore     int                    `json:"average_score" bson:"averageScore"`
	ScoreRange       [2]int                 `json:"score_range" bson:"scoreRange"`
	Themes           []SummaryTheme         `json:"themes" bson:"themes"`
	ActionItems      []SummaryActionItem    `json:"action_items" bson:"actionItems"`
	Contradictions   []SummaryContradiction `json:"contradictions" bson:"contradictions"`
	DimensionTrends  map[string]int         `json:"dimension_trends" bson:"dimensionTrends"`
}

type DomainSummary struct {
	ID               primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	TenantID         string               `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain           string               `json:"domain" bson:"domain"`
	Result           DomainSummaryResult  `json:"result" bson:"result"`
	RawText          string               `json:"raw_text" bson:"rawText"`
	Model            string               `json:"model" bson:"model"`
	OptimizationIDs  []primitive.ObjectID `json:"optimization_ids" bson:"optimizationIds"`
	ReportCount      int                  `json:"report_count" bson:"reportCount"`
	IncludesAnalysis bool                 `json:"includes_analysis" bson:"includesAnalysis"`
	IncludesVideo    bool                 `json:"includes_video" bson:"includesVideo"`
	IncludesReddit   bool                 `json:"includes_reddit" bson:"includesReddit"`
	GeneratedAt      time.Time            `json:"generated_at" bson:"generatedAt"`
}

// Video Authority Analyzer types

type VideoAnalysisConfig struct {
	ChannelURL  string   `json:"channel_url" bson:"channelUrl"`
	VideoURLs   []string `json:"video_urls" bson:"videoUrls"`
	BrandURL    string   `json:"brand_url" bson:"brandUrl"`
	SearchTerms []string `json:"search_terms" bson:"searchTerms"`
}

type YouTubeVideo struct {
	VideoID      string    `json:"video_id" bson:"videoId"`
	Title        string    `json:"title" bson:"title"`
	ChannelTitle string    `json:"channel_title" bson:"channelTitle"`
	ChannelID    string    `json:"channel_id" bson:"channelId"`
	Description  string    `json:"description" bson:"description"`
	PublishedAt  time.Time `json:"published_at" bson:"publishedAt"`
	ViewCount    int64     `json:"view_count" bson:"viewCount"`
	LikeCount    int64     `json:"like_count" bson:"likeCount"`
	CommentCount int64     `json:"comment_count" bson:"commentCount"`
	Duration     string    `json:"duration" bson:"duration"`
	Tags         []string  `json:"tags" bson:"tags"`
	Transcript   string    `json:"transcript,omitempty" bson:"transcript,omitempty"`
	RelevanceTag string    `json:"relevance_tag" bson:"relevanceTag"`
}

type YouTubeChannel struct {
	ChannelID       string `json:"channel_id" bson:"channelId"`
	Title           string `json:"title" bson:"title"`
	SubscriberCount int64  `json:"subscriber_count" bson:"subscriberCount"`
	VideoCount      int64  `json:"video_count" bson:"videoCount"`
	ViewCount       int64  `json:"view_count" bson:"viewCount"`
}

// Video recommendation — structured action item from video analysis
type VideoRecommendation struct {
	Action         string `json:"action" bson:"action"`
	ExpectedImpact string `json:"expected_impact" bson:"expectedImpact"`
	Dimension      string `json:"dimension" bson:"dimension"` // transcript_authority, topical_dominance, citation_network, brand_narrative
	Priority       string `json:"priority" bson:"priority"`   // high, medium, low
	VideoID        string `json:"video_id,omitempty" bson:"videoId,omitempty"`
}

// Per-video scorecard (own channel videos)
type VideoScorecard struct {
	VideoID                  string   `json:"video_id" bson:"videoId"`
	Title                    string   `json:"title" bson:"title"`
	OverallScore             int      `json:"overall_score" bson:"overallScore"`
	TranscriptPower          int      `json:"transcript_power" bson:"transcriptPower"`
	StructuralExtractability int      `json:"structural_extractability" bson:"structuralExtractability"`
	DiscoverySurface         int      `json:"discovery_surface" bson:"discoverySurface"`
	HasTranscript            bool     `json:"has_transcript" bson:"hasTranscript"`
	KeyFindings              []string `json:"key_findings" bson:"keyFindings"`
}

// Brand mention in a third-party video
type BrandMention struct {
	VideoID              string   `json:"video_id" bson:"videoId"`
	Title                string   `json:"title" bson:"title"`
	ChannelTitle         string   `json:"channel_title" bson:"channelTitle"`
	ViewCount            int64    `json:"view_count" bson:"viewCount"`
	Sentiment            string   `json:"sentiment" bson:"sentiment"`
	MentionContext       string   `json:"mention_context" bson:"mentionContext"`
	MentionPosition      string   `json:"mention_position" bson:"mentionPosition"`
	Extractability       string   `json:"extractability" bson:"extractability"`
	CompetitorsMentioned []string `json:"competitors_mentioned" bson:"competitorsMentioned"`
}

// Creator/channel profile assessment
type CreatorProfile struct {
	ChannelTitle    string `json:"channel_title" bson:"channelTitle"`
	ChannelID       string `json:"channel_id" bson:"channelId"`
	SubscriberCount int64  `json:"subscriber_count" bson:"subscriberCount"`
	Sentiment       string `json:"sentiment" bson:"sentiment"`
	VideoCount      int    `json:"video_count" bson:"videoCount"`
	TotalViews      int64  `json:"total_views" bson:"totalViews"`
	Role            string `json:"role" bson:"role"`
	AuthorityScore  int    `json:"authority_score" bson:"authorityScore"`
}

type ShareOfVoiceEntry struct {
	BrandName    string  `json:"brand_name" bson:"brandName"`
	MentionCount int     `json:"mention_count" bson:"mentionCount"`
	Percentage   float64 `json:"percentage" bson:"percentage"`
}

type ContentGap struct {
	Query                string   `json:"query" bson:"query"`
	CompetitorsMentioned []string `json:"competitors_mentioned" bson:"competitorsMentioned"`
	OpportunityScore     int      `json:"opportunity_score" bson:"opportunityScore"`
	VideoCount           int      `json:"video_count" bson:"videoCount"`
	Recommendation       string   `json:"recommendation" bson:"recommendation"`
}

type CreatorTarget struct {
	ChannelTitle         string   `json:"channel_title" bson:"channelTitle"`
	ChannelID            string   `json:"channel_id" bson:"channelId"`
	SubscriberCount      int64    `json:"subscriber_count" bson:"subscriberCount"`
	CategoryRelevance    string   `json:"category_relevance" bson:"categoryRelevance"`
	CompetitorsMentioned []string `json:"competitors_mentioned" bson:"competitorsMentioned"`
	OutreachReason       string   `json:"outreach_reason" bson:"outreachReason"`
}

// Pillar sub-scores for the 4-pillar unified report

type TranscriptAuthorityPillar struct {
	Score                int      `json:"score" bson:"score"`
	Evidence             []string `json:"evidence" bson:"evidence"`
	TranscriptCoverage   int      `json:"transcript_coverage" bson:"transcriptCoverage"`     // % of own videos with transcripts
	KeywordAlignment     int      `json:"keyword_alignment" bson:"keywordAlignment"`         // 0-100
	QuotabilityScore     int      `json:"quotability_score" bson:"quotabilityScore"`         // 0-100
	InformationDensity   int      `json:"information_density" bson:"informationDensity"`     // 0-100
}

type TopicalDominancePillar struct {
	Score          int               `json:"score" bson:"score"`
	Evidence       []string          `json:"evidence" bson:"evidence"`
	TopicsCovered  int               `json:"topics_covered" bson:"topicsCovered"`
	TopicsTotal    int               `json:"topics_total" bson:"topicsTotal"`
	CoverageDepth  int               `json:"coverage_depth" bson:"coverageDepth"` // 0-100
	VsCompetitors  string            `json:"vs_competitors" bson:"vsCompetitors"` // narrative comparison
	ShareOfVoice   []ShareOfVoiceEntry `json:"share_of_voice" bson:"shareOfVoice"`
	ContentGaps    []ContentGap      `json:"content_gaps" bson:"contentGaps"`
}

type CitationNetworkPillar struct {
	Score              int             `json:"score" bson:"score"`
	Evidence           []string        `json:"evidence" bson:"evidence"`
	CreatorMentions    int             `json:"creator_mentions" bson:"creatorMentions"`       // # third-party creators mentioning brand
	AuthoritativeRefs  int             `json:"authoritative_refs" bson:"authoritativeRefs"`   // mentions by high-authority creators
	ConcentrationRisk  string          `json:"concentration_risk" bson:"concentrationRisk"`   // narrative on concentration
	TopCreators        []CreatorProfile `json:"top_creators" bson:"topCreators"`
	CreatorTargets     []CreatorTarget `json:"creator_targets" bson:"creatorTargets"`
}

type SentimentBreakdown struct {
	Positive int `json:"positive" bson:"positive"`
	Neutral  int `json:"neutral" bson:"neutral"`
	Negative int `json:"negative" bson:"negative"`
	Total    int `json:"total" bson:"total"`
}

type BrandNarrativePillar struct {
	Score              int              `json:"score" bson:"score"`
	Evidence           []string         `json:"evidence" bson:"evidence"`
	Sentiment          SentimentBreakdown `json:"sentiment" bson:"sentiment"`
	NarrativeSummary   string           `json:"narrative_summary" bson:"narrativeSummary"`
	NarrativeCoherence int              `json:"narrative_coherence" bson:"narrativeCoherence"` // 0-100
	KeyThemes          []string         `json:"key_themes" bson:"keyThemes"`
	BrandMentions      []BrandMention   `json:"brand_mentions" bson:"brandMentions"`
}

// Unified Video Authority Result — 4-pillar report
type VideoAuthorityResult struct {
	OverallScore        int                       `json:"overall_score" bson:"overallScore"`
	TranscriptAuthority TranscriptAuthorityPillar `json:"transcript_authority" bson:"transcriptAuthority"`
	TopicalDominance    TopicalDominancePillar    `json:"topical_dominance" bson:"topicalDominance"`
	CitationNetwork     CitationNetworkPillar     `json:"citation_network" bson:"citationNetwork"`
	BrandNarrative      BrandNarrativePillar      `json:"brand_narrative" bson:"brandNarrative"`
	VideoScorecards     []VideoScorecard          `json:"video_scorecards" bson:"videoScorecards"`
	ExecutiveSummary    string                    `json:"executive_summary" bson:"executiveSummary"`
	ConfidenceNote      string                    `json:"confidence_note" bson:"confidenceNote"`
	Recommendations     []VideoRecommendation     `json:"recommendations" bson:"recommendations"`
}

// Top-level video analysis document
type VideoAnalysis struct {
	ID               primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	TenantID         string               `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain           string               `json:"domain" bson:"domain"`
	Config           VideoAnalysisConfig  `json:"config" bson:"config"`
	Videos           []YouTubeVideo       `json:"videos" bson:"videos"`
	Result           *VideoAuthorityResult `json:"result,omitempty" bson:"result,omitempty"`
	RawText          string               `json:"raw_text" bson:"rawText"`
	Model            string               `json:"model" bson:"model"`
	BrandContextUsed bool                 `json:"brand_context_used" bson:"brandContextUsed"`
	GeneratedAt      time.Time            `json:"generated_at" bson:"generatedAt"`
}

type VideoAnalysisSummary struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	Domain       string             `json:"domain" bson:"domain"`
	OverallScore *int               `json:"overall_score,omitempty"`
	VideoCount   int                `json:"video_count"`
	Model        string             `json:"model" bson:"model"`
	GeneratedAt  time.Time          `json:"generated_at" bson:"generatedAt"`
}

// Video assessment (Phase 1 of two-phase pipeline)

type VideoAssessment struct {
	VideoID          string   `json:"video_id" bson:"video_id"`
	Title            string   `json:"title" bson:"title"`
	KeywordAlignment int      `json:"keyword_alignment" bson:"keyword_alignment"`
	Quotability      int      `json:"quotability" bson:"quotability"`
	InfoDensity      int      `json:"info_density" bson:"info_density"`
	KeyQuotes        []string `json:"key_quotes" bson:"key_quotes"`
	Topics           []string `json:"topics" bson:"topics"`
	BrandSentiment   string   `json:"brand_sentiment" bson:"brand_sentiment"`
	Summary          string   `json:"summary" bson:"summary"`
	HasTranscript    bool     `json:"has_transcript" bson:"has_transcript"`
}

// Domain sharing

type DomainShare struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID   string             `json:"tenant_id" bson:"tenantId"`
	Domain     string             `json:"domain" bson:"domain"`
	ShareID    string             `json:"share_id" bson:"shareId"`
	Visibility string             `json:"visibility" bson:"visibility"` // "private", "public", "popular"
	ViewCount  int                `json:"view_count" bson:"viewCount"`
	CreatedAt  time.Time          `json:"created_at" bson:"createdAt"`
	UpdatedAt  time.Time          `json:"updated_at" bson:"updatedAt"`
}

// BrandScreenshot stores a captured homepage screenshot for a popular brand.
type BrandScreenshot struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID    string             `json:"tenant_id" bson:"tenantId"`
	Domain      string             `json:"domain" bson:"domain"`
	ImageData   []byte             `json:"-" bson:"imageData"`
	ContentType string             `json:"content_type" bson:"contentType"`
	Width       int                `json:"width" bson:"width"`
	Height      int                `json:"height" bson:"height"`
	SizeBytes   int                `json:"size_bytes" bson:"sizeBytes"`
	CapturedAt  time.Time          `json:"captured_at" bson:"capturedAt"`
	Error       string             `json:"error,omitempty" bson:"error,omitempty"`
}

// Tenant API key management

type TenantAPIKey struct {
	ID             primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID       string             `json:"tenant_id" bson:"tenantId"`
	Provider       string             `json:"provider" bson:"provider"`                              // "anthropic", "openai", "grok", "gemini"
	EncryptedKey   string             `json:"-" bson:"encryptedKey"`
	KeyPrefix      string             `json:"key_prefix" bson:"keyPrefix"`                           // first 8 chars for display
	PreferredModel string             `json:"preferred_model,omitempty" bson:"preferredModel,omitempty"`
	Status         string             `json:"status" bson:"status"`                                  // "active", "invalid", "no_credits"
	LastVerifiedAt *time.Time         `json:"last_verified_at,omitempty" bson:"lastVerifiedAt,omitempty"`
	CreatedAt      time.Time          `json:"created_at" bson:"createdAt"`
	UpdatedAt      time.Time          `json:"updated_at" bson:"updatedAt"`
}

// Tenant settings (primary provider, etc.)

type TenantSettings struct {
	ID              primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID        string             `json:"tenant_id" bson:"tenantId"`
	PrimaryProvider string             `json:"primary_provider" bson:"primaryProvider"` // "anthropic", "openai", "grok", "gemini"
	UpdatedAt       time.Time          `json:"updated_at" bson:"updatedAt"`
}

// YouTube API cache

type YouTubeCache struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CacheKey  string             `json:"cache_key" bson:"cacheKey"`
	Data      string             `json:"data" bson:"data"`
	CachedAt  time.Time          `json:"cached_at" bson:"cachedAt"`
	ExpiresAt time.Time          `json:"expires_at" bson:"expiresAt"`
}

// Reddit Authority Analyzer types

type RedditAnalysisConfig struct {
	Subreddits  []string `json:"subreddits" bson:"subreddits"`
	SearchTerms []string `json:"search_terms" bson:"searchTerms"`
	BrandURL    string   `json:"brand_url" bson:"brandUrl"`
	TimeFilter  string   `json:"time_filter" bson:"timeFilter"` // "month", "year", "all"
}

// RedditThreadSummary is the stored version of a thread (without full comments for space).
type RedditThreadSummary struct {
	ID          string    `json:"id" bson:"id"`
	Subreddit   string    `json:"subreddit" bson:"subreddit"`
	Title       string    `json:"title" bson:"title"`
	SelfText    string    `json:"self_text" bson:"selfText"`
	Author      string    `json:"author" bson:"author"`
	Score       int       `json:"score" bson:"score"`
	UpvoteRatio float64   `json:"upvote_ratio" bson:"upvoteRatio"`
	NumComments int       `json:"num_comments" bson:"numComments"`
	URL         string    `json:"url" bson:"url"`
	Permalink   string    `json:"permalink" bson:"permalink"`
	CreatedUTC  time.Time `json:"created_utc" bson:"createdUtc"`
	IsSelfPost  bool      `json:"is_self_post" bson:"isSelfPost"`
	CommentCount int      `json:"comment_count" bson:"commentCount"` // top comments fetched
}

// Reddit pillar types for the 4-pillar report

type RedditShareOfVoice struct {
	BrandName    string  `json:"brand_name" bson:"brandName"`
	MentionCount int     `json:"mention_count" bson:"mentionCount"`
	Percentage   float64 `json:"percentage" bson:"percentage"`
}

type RedditMentionExample struct {
	ThreadID     string `json:"thread_id" bson:"threadId"`
	Subreddit    string `json:"subreddit" bson:"subreddit"`
	Title        string `json:"title" bson:"title"`
	Score        int    `json:"score" bson:"score"`
	Sentiment    string `json:"sentiment" bson:"sentiment"` // positive, neutral, negative
	Context      string `json:"context" bson:"context"`     // excerpt showing mention
	IsRecommendation bool `json:"is_recommendation" bson:"isRecommendation"`
}

type RedditPresencePillar struct {
	Score            int                  `json:"score" bson:"score"`
	Evidence         []string             `json:"evidence" bson:"evidence"`
	TotalMentions    int                  `json:"total_mentions" bson:"totalMentions"`
	UniqueSubreddits int                  `json:"unique_subreddits" bson:"uniqueSubreddits"`
	ShareOfVoice     []RedditShareOfVoice `json:"share_of_voice" bson:"shareOfVoice"`
	MentionTrend     string               `json:"mention_trend" bson:"mentionTrend"` // growing, stable, declining
}

type RedditSentimentPillar struct {
	Score              int                    `json:"score" bson:"score"`
	Evidence           []string               `json:"evidence" bson:"evidence"`
	Sentiment          SentimentBreakdown     `json:"sentiment" bson:"sentiment"`
	RecommendationRate int                    `json:"recommendation_rate" bson:"recommendationRate"` // % of mentions that recommend
	TopPraise          []string               `json:"top_praise" bson:"topPraise"`
	TopCriticism       []string               `json:"top_criticism" bson:"topCriticism"`
	NotableMentions    []RedditMentionExample `json:"notable_mentions" bson:"notableMentions"`
}

type RedditCompetitivePillar struct {
	Score               int                    `json:"score" bson:"score"`
	Evidence            []string               `json:"evidence" bson:"evidence"`
	WinRate             int                    `json:"win_rate" bson:"winRate"` // % of head-to-head mentions where brand wins
	ComparisonThreads   int                    `json:"comparison_threads" bson:"comparisonThreads"`
	Differentiators     []string               `json:"differentiators" bson:"differentiators"`
	CompetitorStrengths []string               `json:"competitor_strengths" bson:"competitorStrengths"`
	HeadToHeadExamples  []RedditMentionExample `json:"head_to_head_examples" bson:"headToHeadExamples"`
}

type RedditTrainingSignalPillar struct {
	Score             int      `json:"score" bson:"score"`
	Evidence          []string `json:"evidence" bson:"evidence"`
	HighScoreThreads  int      `json:"high_score_threads" bson:"highScoreThreads"`   // threads with 50+ upvotes
	DeepThreads       int      `json:"deep_threads" bson:"deepThreads"`               // threads with 10+ comments
	AuthorityTier     string   `json:"authority_tier" bson:"authorityTier"`           // strong, moderate, weak
	KeyThreads        []RedditMentionExample `json:"key_threads" bson:"keyThreads"` // threads likely to influence LLMs
	Recommendations   []string `json:"recommendations" bson:"recommendations"`
}

// RedditRecommendation is a structured action item from Reddit analysis.
type RedditRecommendation struct {
	Action         string `json:"action" bson:"action"`
	ExpectedImpact string `json:"expected_impact" bson:"expectedImpact"`
	Dimension      string `json:"dimension" bson:"dimension"` // presence, sentiment, competitive, training_signal
	Priority       string `json:"priority" bson:"priority"`   // high, medium, low
}

type RedditAuthorityResult struct {
	OverallScore     int                        `json:"overall_score" bson:"overallScore"`
	Presence         RedditPresencePillar       `json:"presence" bson:"presence"`
	Sentiment        RedditSentimentPillar      `json:"sentiment" bson:"sentiment"`
	Competitive      RedditCompetitivePillar    `json:"competitive" bson:"competitive"`
	TrainingSignal   RedditTrainingSignalPillar `json:"training_signal" bson:"trainingSignal"`
	ExecutiveSummary string                     `json:"executive_summary" bson:"executiveSummary"`
	ConfidenceNote   string                     `json:"confidence_note" bson:"confidenceNote"`
	Recommendations  []RedditRecommendation     `json:"recommendations" bson:"recommendations"`
}

// Top-level Reddit analysis document
type RedditAnalysis struct {
	ID               primitive.ObjectID      `json:"id" bson:"_id,omitempty"`
	TenantID         string                  `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain           string                  `json:"domain" bson:"domain"`
	Config           RedditAnalysisConfig    `json:"config" bson:"config"`
	Threads          []RedditThreadSummary   `json:"threads" bson:"threads"`
	Result           *RedditAuthorityResult  `json:"result,omitempty" bson:"result,omitempty"`
	RawText          string                  `json:"raw_text" bson:"rawText"`
	Model            string                  `json:"model" bson:"model"`
	BrandContextUsed bool                    `json:"brand_context_used" bson:"brandContextUsed"`
	GeneratedAt      time.Time               `json:"generated_at" bson:"generatedAt"`
}

type RedditCache struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CacheKey  string             `json:"cache_key" bson:"cacheKey"`
	Data      string             `json:"data" bson:"data"`
	CachedAt  time.Time          `json:"cached_at" bson:"cachedAt"`
	ExpiresAt time.Time          `json:"expires_at" bson:"expiresAt"`
}

// --- Search Visibility Analysis models ---

type SearchVisibilityResult struct {
	OverallScore      int                        `json:"overall_score" bson:"overallScore"`
	AIOReadiness      AIOReadinessPillar         `json:"aio_readiness" bson:"aioReadiness"`
	CrawlAccess       CrawlAccessibilityPillar   `json:"crawl_accessibility" bson:"crawlAccessibility"`
	BrandMomentum     BrandSearchMomentumPillar  `json:"brand_momentum" bson:"brandMomentum"`
	ContentFreshness  ContentFreshnessPillar     `json:"content_freshness" bson:"contentFreshness"`
	CategoryDiscovery *CategoryDiscoveryPillar   `json:"category_discovery,omitempty" bson:"categoryDiscovery,omitempty"`
	ExecutiveSummary  string                     `json:"executive_summary" bson:"executiveSummary"`
	ConfidenceNote    string                     `json:"confidence_note" bson:"confidenceNote"`
	Recommendations   []SearchRecommendation     `json:"recommendations" bson:"recommendations"`
}

type AIOReadinessPillar struct {
	Score             int      `json:"score" bson:"score"`
	Evidence          []string `json:"evidence" bson:"evidence"`
	OrganicPresence   int      `json:"organic_presence" bson:"organicPresence"`       // 0-100: estimated presence in top results
	StructuredData    int      `json:"structured_data" bson:"structuredData"`         // 0-100: Schema.org / rich markup quality
	ContentFormat     int      `json:"content_format" bson:"contentFormat"`           // 0-100: alignment with AIO-preferred formats
	AnswerProminence  int      `json:"answer_prominence" bson:"answerProminence"`     // 0-100: front-loaded, concise answers
}

type CrawlAccessibilityPillar struct {
	Score              int      `json:"score" bson:"score"`
	Evidence           []string `json:"evidence" bson:"evidence"`
	RobotsTxtPolicy    string   `json:"robots_txt_policy" bson:"robotsTxtPolicy"`       // e.g. "allows all AI crawlers", "blocks GPTBot", etc.
	AIBotAccess        int      `json:"ai_bot_access" bson:"aiBotAccess"`               // 0-100: how open to AI crawlers
	SitemapQuality     int      `json:"sitemap_quality" bson:"sitemapQuality"`           // 0-100
	RenderAccessibility int     `json:"render_accessibility" bson:"renderAccessibility"` // 0-100: JS-rendering, accessibility
	CrawlerDetails     []CrawlerStatus `json:"crawler_details" bson:"crawlerDetails"`
}

type CrawlerStatus struct {
	Name    string `json:"name" bson:"name"`       // e.g. "GPTBot", "ClaudeBot", "PerplexityBot"
	Allowed bool   `json:"allowed" bson:"allowed"`
	Notes   string `json:"notes" bson:"notes"`
}

type BrandSearchMomentumPillar struct {
	Score             int      `json:"score" bson:"score"`
	Evidence          []string `json:"evidence" bson:"evidence"`
	BrandSearchTrend  string   `json:"brand_search_trend" bson:"brandSearchTrend"` // "growing", "stable", "declining"
	CompetitorCompare string   `json:"competitor_compare" bson:"competitorCompare"` // narrative
	WebMentionStrength int     `json:"web_mention_strength" bson:"webMentionStrength"` // 0-100
	EntityRecognition  int     `json:"entity_recognition" bson:"entityRecognition"`   // 0-100: how well-known the brand entity is
}

type ContentFreshnessPillar struct {
	Score              int      `json:"score" bson:"score"`
	Evidence           []string `json:"evidence" bson:"evidence"`
	AverageContentAge  string   `json:"average_content_age" bson:"averageContentAge"` // narrative (e.g. "Most content is 6-12 months old")
	UpdateFrequency    string   `json:"update_frequency" bson:"updateFrequency"`     // "frequent", "moderate", "infrequent", "stale"
	FreshnessSignals   int      `json:"freshness_signals" bson:"freshnessSignals"`   // 0-100: presence of dates, last-modified, etc.
	ContentDecayRisk   int      `json:"content_decay_risk" bson:"contentDecayRisk"`  // 0-100: how many key pages are aging out
}

type SearchRecommendation struct {
	Action         string `json:"action" bson:"action"`
	Priority       string `json:"priority" bson:"priority"` // "high", "medium", "low"
	ExpectedImpact string `json:"expected_impact" bson:"expectedImpact"`
	Dimension      string `json:"dimension" bson:"dimension"` // which pillar
}

type CategoryDiscoveryPillar struct {
	Score              int      `json:"score" bson:"score"`
	Evidence           []string `json:"evidence" bson:"evidence"`
	CategoryVisibility int      `json:"category_visibility" bson:"categoryVisibility"`
	IntentCoverage     int      `json:"intent_coverage" bson:"intentCoverage"`
	CompetitorGap      int      `json:"competitor_gap" bson:"competitorGap"`
	DiscoveryPotential int      `json:"discovery_potential" bson:"discoveryPotential"`
}

type SearchAnalysis struct {
	ID               primitive.ObjectID       `json:"id" bson:"_id,omitempty"`
	TenantID         string                   `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain           string                   `json:"domain" bson:"domain"`
	Result           *SearchVisibilityResult  `json:"result,omitempty" bson:"result,omitempty"`
	RawText          string                   `json:"raw_text" bson:"rawText"`
	Model            string                   `json:"model" bson:"model"`
	BrandContextUsed bool                     `json:"brand_context_used" bson:"brandContextUsed"`
	GeneratedAt      time.Time                `json:"generated_at" bson:"generatedAt"`
}

type SearchAnalysisSummary struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	Domain       string             `json:"domain" bson:"domain"`
	OverallScore *int               `json:"overall_score,omitempty"`
	Model        string             `json:"model" bson:"model"`
	GeneratedAt  time.Time          `json:"generated_at" bson:"generatedAt"`
}

// FailedAnalysis records a failed analysis attempt for user visibility.
// Stored in a separate collection with 7-day TTL auto-expiry.
type FailedAnalysis struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID     string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain       string             `json:"domain" bson:"domain"`
	FeedType     string             `json:"feed_type" bson:"feedType"`       // "site","video","reddit","search","summary"
	ErrorCode    string             `json:"error_code" bson:"errorCode"`     // e.g. "api_overloaded", "api_error"
	ErrorMessage string             `json:"error_message" bson:"errorMessage"`
	Model        string             `json:"model,omitempty" bson:"model,omitempty"`
	FailedAt     time.Time          `json:"failed_at" bson:"failedAt"`
}

// ReportPDF stores a cached aggregate PDF report for a domain.
type ReportPDF struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TenantID    string             `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain      string             `json:"domain" bson:"domain"`
	PDFData     []byte             `json:"-" bson:"pdfData"`
	SizeBytes   int                `json:"size_bytes" bson:"sizeBytes"`
	Fingerprint ReportFingerprint  `json:"fingerprint" bson:"fingerprint"`
	GeneratedAt time.Time          `json:"generated_at" bson:"generatedAt"`
}

// ReportFingerprint captures the latest timestamps from each data source.
// If any source timestamp changes, the cached PDF is stale.
type ReportFingerprint struct {
	LatestOptimizationAt *time.Time `json:"latest_optimization_at,omitempty" bson:"latestOptimizationAt,omitempty"`
	AnalysisCreatedAt    *time.Time `json:"analysis_created_at,omitempty" bson:"analysisCreatedAt,omitempty"`
	VideoGeneratedAt     *time.Time `json:"video_generated_at,omitempty" bson:"videoGeneratedAt,omitempty"`
	RedditGeneratedAt    *time.Time `json:"reddit_generated_at,omitempty" bson:"redditGeneratedAt,omitempty"`
	SummaryGeneratedAt   *time.Time `json:"summary_generated_at,omitempty" bson:"summaryGeneratedAt,omitempty"`
	SearchGeneratedAt    *time.Time `json:"search_generated_at,omitempty" bson:"searchGeneratedAt,omitempty"`
	LatestTodoAt         *time.Time `json:"latest_todo_at,omitempty" bson:"latestTodoAt,omitempty"`
	OptimizationCount    int        `json:"optimization_count" bson:"optimizationCount"`
	AnalysisExists       bool       `json:"analysis_exists" bson:"analysisExists"`
	VideoExists          bool       `json:"video_exists" bson:"videoExists"`
	RedditExists         bool       `json:"reddit_exists" bson:"redditExists"`
	SearchExists         bool       `json:"search_exists" bson:"searchExists"`
}

// LLM Test types — multi-provider brand knowledge testing

type LLMTestQuery struct {
	Query    string `json:"query" bson:"query"`
	Type     string `json:"type" bson:"type"`         // brand, category, comparison, discovery, custom
	Priority string `json:"priority" bson:"priority"` // high, medium, low
}

type LLMTestProviderResult struct {
	ProviderID   string `json:"provider_id" bson:"providerId"`
	ProviderName string `json:"provider_name" bson:"providerName"`
	Model        string `json:"model" bson:"model"`
	Response     string `json:"response" bson:"response"`
	Mentioned    bool   `json:"mentioned" bson:"mentioned"`
	Recommended  bool   `json:"recommended" bson:"recommended"`
	Sentiment    string `json:"sentiment" bson:"sentiment"` // positive, neutral, negative, absent
	Accuracy     string `json:"accuracy" bson:"accuracy"`   // accurate, partially_accurate, inaccurate, not_applicable
	Score        int    `json:"score" bson:"score"`
}

type LLMTestQueryResult struct {
	Query           LLMTestQuery            `json:"query" bson:"query"`
	ProviderResults []LLMTestProviderResult `json:"provider_results" bson:"providerResults"`
}

type LLMTestSummary struct {
	ProviderID     string `json:"provider_id" bson:"providerId"`
	ProviderName   string `json:"provider_name" bson:"providerName"`
	Model          string `json:"model" bson:"model"`
	OverallScore   int    `json:"overall_score" bson:"overallScore"`
	MentionRate    int    `json:"mention_rate" bson:"mentionRate"`
	RecommendRate  int    `json:"recommend_rate" bson:"recommendRate"`
	AccuracyRate   int    `json:"accuracy_rate" bson:"accuracyRate"`
	SentimentScore int    `json:"sentiment_score" bson:"sentimentScore"`
	// Answered and Failed count this provider's queries; rates are over Answered.
	Answered int `json:"answered,omitempty" bson:"answered,omitempty"`
	Failed   int `json:"failed,omitempty" bson:"failed,omitempty"`
}

// LLMTestUnavailable records a selected provider that couldn't answer any query,
// so it is left out of the scores instead of counting as zero.
type LLMTestUnavailable struct {
	ProviderID   string `json:"provider_id" bson:"providerId"`
	ProviderName string `json:"provider_name" bson:"providerName"`
	Model        string `json:"model,omitempty" bson:"model,omitempty"`
	Reason       string `json:"reason" bson:"reason"`
}

type LLMTest struct {
	ID                primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	TenantID          string               `json:"tenant_id,omitempty" bson:"tenantId,omitempty"`
	Domain            string               `json:"domain" bson:"domain"`
	BrandName         string               `json:"brand_name" bson:"brandName"`
	RunNumber         int                  `json:"run_number" bson:"runNumber"`
	Queries           []LLMTestQuery       `json:"queries" bson:"queries"`
	Results           []LLMTestQueryResult `json:"results" bson:"results"`
	ProviderSummaries []LLMTestSummary     `json:"provider_summaries" bson:"providerSummaries"`
	// UnavailableProviders lists selected providers that couldn't answer at all.
	UnavailableProviders []LLMTestUnavailable `json:"unavailable_providers,omitempty" bson:"unavailableProviders,omitempty"`
	OverallScore         int                  `json:"overall_score" bson:"overallScore"`
	BrandContextUsed     bool                 `json:"brand_context_used" bson:"brandContextUsed"`
	CompetitorOf         string               `json:"competitor_of,omitempty" bson:"competitorOf,omitempty"`
	GeneratedAt          time.Time            `json:"generated_at" bson:"generatedAt"`
}

// UserAccessKey is a per-user API key for accessing llmopt's MCP server.
// Keys use the "lok_" prefix and are stored as SHA-256 hashes — the raw key
// is returned only once at creation time.
type UserAccessKey struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID     string             `json:"user_id" bson:"userId"`
	TenantID   string             `json:"tenant_id" bson:"tenantId"`
	Name       string             `json:"name" bson:"name"`
	KeyHash    string             `json:"-" bson:"keyHash"`   // SHA-256 hex, never exposed
	KeyPreview string             `json:"key_preview" bson:"keyPreview"` // e.g. "lok_...abc123"
	CreatedAt  time.Time          `json:"created_at" bson:"createdAt"`
	LastUsedAt *time.Time         `json:"last_used_at,omitempty" bson:"lastUsedAt,omitempty"`
	IsActive   bool               `json:"is_active" bson:"isActive"`
}
