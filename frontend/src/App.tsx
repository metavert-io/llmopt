import { useState, useRef, useCallback, useEffect, useMemo } from 'react'
import { apiFetch } from './apiFetch'
import { fetchCurrentUser, fetchUnreadCount, clearAuth, type UserInfo } from './auth'
import { trackPageView, trackEvent, trackEventImmediate, setTelemetryContext } from './telemetry'

interface CrawledPage {
  url: string
  title: string
}

interface Question {
  question: string
  relevance: string
  category: string
  page_urls?: string[]
  brand_status?: string
}

interface AnalysisResult {
  site_summary: string
  questions: Question[]
  crawled_pages?: CrawledPage[]
}

interface AnalysisSummary {
  id: string
  domain: string
  site_summary: string
  question_count: number
  page_count: number
  model: string
  brand_context_used: boolean
  brand_profile_updated_at?: string
  created_at: string
}

interface ResultMeta {
  id: string
  model: string
  createdAt: string
  cached: boolean
  brandContextUsed: boolean
  brandProfileUpdatedAt?: string
}

interface DimensionScore {
  score: number
  evidence: string[]
  improvements: string[]
}

interface Competitor {
  domain: string
  score_estimate: number
  strengths: string
}

interface OptRecommendation {
  priority: string
  action: string
  expected_impact: string
  dimension: string
}

interface OptimizationResult {
  overall_score: number
  content_authority: DimensionScore
  structural_optimization: DimensionScore
  source_authority: DimensionScore
  knowledge_persistence: DimensionScore
  competitors: Competitor[]
  recommendations: OptRecommendation[]
}

interface OptimizationMeta {
  id: string
  model: string
  createdAt: string
  cached: boolean
  question: string
  questionIndex: number
  brandStatus?: string
  brandContextUsed: boolean
  brandProfileUpdatedAt?: string
}

interface TodoItem {
  id: string
  optimization_id: string
  analysis_id: string
  video_analysis_id?: string
  source_type?: string
  domain: string
  question: string
  action: string
  summary?: string
  expected_impact: string
  dimension: string
  priority: string
  status: 'todo' | 'completed' | 'backlogged' | 'archived'
  created_at: string
  completed_at?: string
  backlogged_at?: string
  archived_at?: string
  tags?: string[]
}

type TodoSubTab = 'todo' | 'completed' | 'backlogged' | 'archived'
type TodoSortMode = 'priority' | 'question' | 'dimension'

interface OptimizationListItem {
  id: string
  domain: string
  question: string
  question_index: number
  overall_score: number
  model: string
  public: boolean
  brand_status?: string
  brand_context_used: boolean
  brand_profile_updated_at?: string
  created_at: string
}

interface FullOptimization {
  id: string
  analysis_id: string
  question_index: number
  question: string
  domain: string
  page_urls: string[]
  result: OptimizationResult
  raw_text: string
  model: string
  public: boolean
  brand_status?: string
  brand_context_used: boolean
  brand_profile_updated_at?: string
  created_at: string
}

interface SummaryTheme {
  title: string
  description: string
  report_refs: string[]
}
interface SummaryActionItem {
  priority: string
  action: string
  dimension: string
  expected_impact: string
  source_reports: string[]
}
interface SummaryContradiction {
  topic: string
  positions: string[]
  report_refs: string[]
  recommendation: string
}
interface DomainSummaryResult {
  executive_summary: string
  average_score: number
  score_range: [number, number]
  themes: SummaryTheme[]
  action_items: SummaryActionItem[]
  contradictions: SummaryContradiction[]
  dimension_trends: Record<string, number>
}
interface DomainSummary {
  id: string
  domain: string
  result: DomainSummaryResult
  model: string
  optimization_ids: string[]
  report_count: number
  includes_analysis: boolean
  includes_video: boolean
  includes_reddit: boolean
  generated_at: string
}
interface HealthTimelineRecord {
  id: string
  models: { model: string; name: string; status: string; latency_ms?: number }[]
  checked_at: string
}

// Brand Intelligence types

interface BrandCompetitor {
  name: string
  url: string
  relationship: string
  notes: string
}

interface TargetQuery {
  query: string
  priority: string
  type: string
}

interface KeyMessage {
  claim: string
  evidence_url: string
  priority: string
}

interface BrandPresence {
  youtube_url: string
  subreddits: string[]
  review_site_urls: string[]
  social_profiles: string[]
  content_assets: string[]
  podcasts: string[]
}

interface BrandProfile {
  id: string
  domain: string
  brand_name: string
  description: string
  categories: string[]
  products: string[]
  primary_audience: string
  key_use_cases: string[]
  competitors: BrandCompetitor[]
  target_queries: TargetQuery[]
  key_messages: KeyMessage[]
  differentiators: string[]
  presence: BrandPresence
  presence_complete: boolean
  public: boolean
  last_discovery_at?: string
  created_at: string
  updated_at: string
  video_search_additions?: string[]
  video_search_removals?: string[]
}

interface BrandProfileSummary {
  id: string
  domain: string
  brand_name: string
  competitor_count: number
  query_count: number
  completeness: number
  public: boolean
  updated_at: string
  brand_content_updated_at?: string
}

interface DiscoveredCompetitor {
  name: string
  url: string
  relationship: string
  source: string
  confidence: number
  notes?: string
}

interface SuggestedQuery {
  query: string
  priority: string
  type: string
}

interface SuggestedClaim {
  claim: string
  evidence_url: string
  priority: string
}

type SuggestedDifferentiator = string

// Video Authority types

interface YouTubeVideo {
  video_id: string
  title: string
  channel_title: string
  channel_id: string
  description: string
  published_at: string
  view_count: number
  like_count: number
  comment_count: number
  duration: string
  tags: string[]
  transcript?: string
  relevance_tag: string
}

interface VideoRecommendation {
  action: string
  expected_impact: string
  dimension: string
  priority: string
  video_id?: string
}

interface VideoScorecard {
  video_id: string
  title: string
  overall_score: number
  transcript_power: number
  structural_extractability: number
  discovery_surface: number
  has_transcript: boolean
  key_findings: string[]
}

interface BrandMention {
  video_id: string
  title: string
  channel_title: string
  view_count: number
  sentiment: string
  mention_context: string
  mention_position: string
  extractability: string
  competitors_mentioned: string[]
}

interface CreatorProfile {
  channel_title: string
  channel_id: string
  subscriber_count: number
  sentiment: string
  video_count: number
  total_views: number
  role: string
  authority_score: number
}

interface ShareOfVoiceEntry {
  brand_name: string
  mention_count: number
  percentage: number
}

interface ContentGap {
  query: string
  competitors_mentioned: string[]
  opportunity_score: number
  video_count: number
  recommendation: string
}

interface CreatorTarget {
  channel_title: string
  channel_id: string
  subscriber_count: number
  category_relevance: string
  competitors_mentioned: string[]
  outreach_reason: string
}

interface TranscriptAuthorityPillar {
  score: number
  evidence: string[]
  transcript_coverage: number
  keyword_alignment: number
  quotability_score: number
  information_density: number
}

interface TopicalDominancePillar {
  score: number
  evidence: string[]
  topics_covered: number
  topics_total: number
  coverage_depth: number
  vs_competitors: string
  share_of_voice: ShareOfVoiceEntry[]
  content_gaps: ContentGap[]
}

interface CitationNetworkPillar {
  score: number
  evidence: string[]
  creator_mentions: number
  authoritative_refs: number
  concentration_risk: string
  top_creators: CreatorProfile[]
  creator_targets: CreatorTarget[]
}

interface SentimentBreakdown {
  positive: number
  neutral: number
  negative: number
  total: number
}

interface BrandNarrativePillar {
  score: number
  evidence: string[]
  sentiment: SentimentBreakdown
  narrative_summary: string
  narrative_coherence: number
  key_themes: string[]
  brand_mentions: BrandMention[]
}

interface VideoAuthorityResult {
  overall_score: number
  transcript_authority: TranscriptAuthorityPillar
  topical_dominance: TopicalDominancePillar
  citation_network: CitationNetworkPillar
  brand_narrative: BrandNarrativePillar
  video_scorecards: VideoScorecard[]
  executive_summary: string
  confidence_note: string
  recommendations: VideoRecommendation[]
}

interface VideoAnalysis {
  id: string
  domain: string
  config: { channel_url: string; video_urls: string[]; brand_url: string; search_terms: string[] }
  videos: YouTubeVideo[]
  result?: VideoAuthorityResult
  model: string
  brand_context_used: boolean
  generated_at: string
}

interface VideoAnalysisSummary {
  id: string
  domain: string
  overall_score?: number
  video_count: number
  model: string
  generated_at: string
}

interface VideoDetail {
  video_id: string
  title: string
  transcript: string
  transcript_length: number
  assessment: {
    keyword_alignment: number
    quotability: number
    info_density: number
    key_quotes: string[]
    topics: string[]
    brand_sentiment: string
    summary: string
  } | null
}

type VideoView = 'input' | 'discovering' | 'review' | 'running' | 'results' | 'transcripts'

type OptimizeView = 'list' | 'detail' | 'running'

interface ModelStatus {
  provider?: string
  model: string
  name: string
  status: 'available' | 'overloaded' | 'error'
  latency_ms?: number
  http_status?: number
  error?: string
}

interface HealthCheck {
  models: ModelStatus[]
  checked_at: string
}

// Reddit Authority types

interface RedditMentionExample {
  thread_id: string
  subreddit: string
  title: string
  score: number
  sentiment: string
  context: string
  is_recommendation: boolean
}

interface RedditShareOfVoice {
  brand_name: string
  mention_count: number
  percentage: number
}

interface RedditPresencePillar {
  score: number
  evidence: string[]
  total_mentions: number
  unique_subreddits: number
  share_of_voice: RedditShareOfVoice[]
  mention_trend: string
}

interface RedditSentimentPillar {
  score: number
  evidence: string[]
  sentiment: SentimentBreakdown
  recommendation_rate: number
  top_praise: string[]
  top_criticism: string[]
  notable_mentions: RedditMentionExample[]
}

interface RedditCompetitivePillar {
  score: number
  evidence: string[]
  win_rate: number
  comparison_threads: number
  differentiators: string[]
  competitor_strengths: string[]
  head_to_head_examples: RedditMentionExample[]
}

interface RedditTrainingSignalPillar {
  score: number
  evidence: string[]
  high_score_threads: number
  deep_threads: number
  authority_tier: string
  key_threads: RedditMentionExample[]
  recommendations: string[]
}

interface RedditRecommendation {
  action: string
  expected_impact: string
  dimension: string
  priority: string
}

interface RedditAuthorityResult {
  overall_score: number
  presence: RedditPresencePillar
  sentiment: RedditSentimentPillar
  competitive: RedditCompetitivePillar
  training_signal: RedditTrainingSignalPillar
  executive_summary: string
  confidence_note: string
  recommendations: RedditRecommendation[]
}

interface RedditThreadSummary {
  id: string
  subreddit: string
  title: string
  self_text?: string
  author: string
  score: number
  upvote_ratio: number
  num_comments: number
  url: string
  permalink: string
  created_utc: string
  is_self_post: boolean
  comment_count?: number
}

interface RedditAnalysis {
  id: string
  domain: string
  config: { subreddits: string[]; search_terms: string[]; brand_url: string; time_filter: string }
  threads: RedditThreadSummary[]
  result?: RedditAuthorityResult
  model: string
  brand_context_used: boolean
  generated_at: string
}

interface RedditAnalysisSummary {
  id: string
  domain: string
  overall_score?: number
  thread_count: number
  model: string
  generated_at: string
}

type RedditView = 'input' | 'discovering' | 'review' | 'running' | 'results'

interface SearchVisibilityResult {
  overall_score: number
  aio_readiness: { score: number; evidence: string[]; organic_presence: number; structured_data: number; content_format: number; answer_prominence: number }
  crawl_accessibility: { score: number; evidence: string[]; robots_txt_policy: string; ai_bot_access: number; sitemap_quality: number; render_accessibility: number; crawler_details: { name: string; allowed: boolean; notes: string }[] }
  brand_momentum: { score: number; evidence: string[]; brand_search_trend: string; competitor_compare: string; web_mention_strength: number; entity_recognition: number }
  content_freshness: { score: number; evidence: string[]; average_content_age: string; update_frequency: string; freshness_signals: number; content_decay_risk: number }
  category_discovery?: { score: number; evidence: string[]; category_visibility: number; intent_coverage: number; competitor_gap: number; discovery_potential: number }
  executive_summary: string
  confidence_note: string
  recommendations: { action: string; priority: string; expected_impact: string; dimension: string }[]
}

interface SearchAnalysis {
  id: string
  domain: string
  result?: SearchVisibilityResult
  model: string
  brand_context_used: boolean
  generated_at: string
}

interface SearchAnalysisSummary {
  id: string
  domain: string
  overall_score?: number
  model: string
  generated_at: string
}

interface FailedAnalysis {
  id: string
  domain: string
  feed_type: string
  error_code: string
  error_message: string
  model?: string
  failed_at: string
}

// LLM Test types
interface TestQuery {
  query: string
  type: string   // brand, category, comparison, discovery, custom
  priority: string // high, medium, low
}

interface TestProviderResult {
  provider_id: string
  provider_name: string
  model: string
  response: string
  mentioned: boolean
  recommended: boolean
  sentiment: string  // positive, neutral, negative, absent
  accuracy: string   // accurate, partially_accurate, inaccurate, not_applicable
  score: number
}

interface TestQueryResult {
  query: TestQuery
  provider_results: TestProviderResult[]
}

interface TestProviderSummary {
  provider_id: string
  provider_name: string
  model: string
  overall_score: number
  mention_rate: number
  recommend_rate: number
  accuracy_rate: number
  sentiment_score: number
}

interface LLMTestResult {
  id: string
  domain: string
  brand_name: string
  run_number: number
  queries: TestQuery[]
  results: TestQueryResult[]
  provider_summaries: TestProviderSummary[]
  overall_score: number
  brand_context_used: boolean
  competitor_of?: string
  generated_at: string
}

type AppState = 'idle' | 'analyzing' | 'done' | 'error'
type ActiveTab = 'analyze' | 'status' | 'optimize' | 'todos' | 'brand' | 'video' | 'reddit' | 'search' | 'test'

const CATEGORY_COLORS = [
  'bg-primary-500/20 text-primary-300 border-primary-500/30',
  'bg-accent-purple/20 text-purple-300 border-accent-purple/30',
  'bg-accent-cyan/20 text-cyan-300 border-accent-cyan/30',
  'bg-accent-emerald/20 text-emerald-300 border-accent-emerald/30',
  'bg-amber-500/20 text-amber-300 border-amber-500/30',
  'bg-accent-pink/20 text-pink-300 border-accent-pink/30',
  'bg-sky-500/20 text-sky-300 border-sky-500/30',
  'bg-rose-500/20 text-rose-300 border-rose-500/30',
]

function safePathname(urlStr: string): string {
  try {
    const parsed = new URL(urlStr)
    return parsed.pathname === '/' ? urlStr : parsed.pathname
  } catch {
    return urlStr
  }
}

/** Format a date string as "Mon DD, h:mm AM" */
function fmtDate(d: string) {
  return new Date(d).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })
}

/** Human-readable label for error codes */
function errorCodeLabel(code: string): string {
  switch (code) {
    case 'api_overloaded': return 'API overloaded'
    case 'api_error': return 'Upstream API error'
    case 'stream_stalled': return 'API timed out'
    case 'parse_error': return 'Parse error'
    case 'cancelled': return 'Cancelled'
    default: return 'Error'
  }
}

/** Score color helpers (80/60/40 thresholds) */
function scoreTextColor(s: number): string {
  return s >= 80 ? 'text-emerald-400' : s >= 60 ? 'text-amber-400' : s >= 40 ? 'text-orange-400' : 'text-red-400'
}
function scoreBgSolid(s: number): string {
  return s >= 80 ? 'bg-emerald-500' : s >= 60 ? 'bg-amber-500' : s >= 40 ? 'bg-orange-500' : 'bg-red-500'
}
function scoreBadge(s: number): string {
  return s >= 80 ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
    : s >= 60 ? 'bg-amber-500/15 text-amber-400 border-amber-500/30'
    : s >= 40 ? 'bg-orange-500/15 text-orange-400 border-orange-500/30'
    : 'bg-red-500/15 text-red-400 border-red-500/30'
}
function scoreLabel(s: number): string {
  return s >= 80 ? 'Strong Position' : s >= 60 ? 'Moderate — Room for Improvement' : s >= 40 ? 'Weak — Significant Improvements Needed' : 'Poor — Immediate Action Required'
}

/** Extract the hostname from a domain/URL string for grouping and comparison */
function domainKey(input: string): string {
  if (!input) return ''
  let s = input.trim().toLowerCase()
  s = s.replace(/^https?:\/\//, '').replace(/\/.*$/, '').replace(/:\d+$/, '')
  return s
}

/** Normalize a URL: add https:// if missing, strip trailing slash */
function normalizeUrl(input: string): string {
  let s = input.trim().toLowerCase()
  if (!s) return ''
  s = s.replace(/^https?:\/\//, '')
  s = s.replace(/\/+$/, '')
  return s
}

export default function App() {
  const [url, setUrl] = useState('')
  const [state, setState] = useState<AppState>('idle')
  const [statusMessages, setStatusMessages] = useState<string[]>([])
  const [result, setResult] = useState<AnalysisResult | null>(null)
  const [error, setError] = useState('')
  const abortRef = useRef<AbortController | null>(null)

  const [activeTab, setActiveTab] = useState<ActiveTab>(() => {
    const params = new URLSearchParams(window.location.search)
    const tab = params.get('tab')
    const validTabs: ActiveTab[] = ['analyze', 'status', 'optimize', 'todos', 'brand', 'video', 'reddit', 'search', 'test']
    if (tab && validTabs.includes(tab as ActiveTab)) return tab as ActiveTab
    return 'analyze'
  })
  const [selectedDomain, setSelectedDomain] = useState('')
  const [history, setHistory] = useState<AnalysisSummary[]>([])

  const [resultMeta, setResultMeta] = useState<ResultMeta | null>(null)
  const [forceAnalyze, setForceAnalyze] = useState(false)

  const [optimization, setOptimization] = useState<OptimizationResult | null>(null)
  const [optimizationMeta, setOptimizationMeta] = useState<OptimizationMeta | null>(null)
  const [optimizing, setOptimizing] = useState(false)
  const [optimizeAutoArchive, setOptimizeAutoArchive] = useState(true)
  const [optimizeMessages, setOptimizeMessages] = useState<string[]>([])
  const [optimizeError, setOptimizeError] = useState('')
  const [optScores, setOptScores] = useState<Record<number, number>>({})
  const optimizeAbortRef = useRef<AbortController | null>(null)

  const [optList, setOptList] = useState<OptimizationListItem[]>([])
  const [optListLoading, setOptListLoading] = useState(false)
  const [optimizeView, setOptimizeView] = useState<OptimizeView>('list')
  const [selectedOpt, setSelectedOpt] = useState<FullOptimization | null>(null)
  const [optTodos, setOptTodos] = useState<TodoItem[]>([])

  // Domain summary state
  const [activeSummary, setActiveSummary] = useState<DomainSummary | null>(null)
  const [activeSummaryStale, setActiveSummaryStale] = useState(false)
  const [generatingSummary, setGeneratingSummary] = useState(false)
  const [summaryMessages, setSummaryMessages] = useState<string[]>([])

  const [healthHistory, setHealthHistory] = useState<HealthCheck[]>([])
  const [healthChecking, setHealthChecking] = useState(false)

  const [todos, setTodos] = useState<TodoItem[]>([])
  const [todosLoading, setTodosLoading] = useState(false)
  const [todoSubTab, setTodoSubTab] = useState<TodoSubTab>('todo')
  const [todoSortMode, setTodoSortMode] = useState<TodoSortMode>('priority')
  const [expandedTodos, setExpandedTodos] = useState<Set<string>>(new Set())

  const [healthTimeline, setHealthTimeline] = useState<HealthTimelineRecord[]>([])
  const [healthTimelineLoading, setHealthTimelineLoading] = useState(false)

  // Domain sharing state
  const [shareModalDomain, setShareModalDomain] = useState<string | null>(null)
  const [domainShareState, setDomainShareState] = useState<{ visibility: string; share_id: string; share_url: string; view_count?: number } | null>(null)
  const [shareLoading, setShareLoading] = useState(false)
  const [shareCopied, setShareCopied] = useState(false)
  const [popularDomains, setPopularDomains] = useState<{ domain: string; brand_name: string; share_id: string; avg_score: number; report_count: number; analysis_count?: number; has_video?: boolean; has_screenshot?: boolean }[]>([])

  // Research page mode (/research URL)
  const isResearchURL = window.location.pathname === '/research'
  const [showResearch, setShowResearch] = useState(isResearchURL)

  // API docs page mode (/docs URL)
  const isDocsURL = window.location.pathname === '/docs'
  const [showDocs, setShowDocs] = useState(isDocsURL)

  // Shared view mode (read-only /share/{shareId} URL)
  const isShareURL = /^\/share\/[A-Za-z0-9]+$/.test(window.location.pathname)
  const [sharedMode, setSharedMode] = useState(isShareURL)
  const [sharedLoading, setSharedLoading] = useState(false)
  const [sharedNotFound, setSharedNotFound] = useState(false)
  const sharedModeRef = useRef(isShareURL)
  const [sharedShareId, setSharedShareId] = useState('')
  const [sharedOptimizations, setSharedOptimizations] = useState<FullOptimization[]>([])

  // Batch optimize
  const [batchOptimizing, setBatchOptimizing] = useState(false)
  const [batchOptProgress, setBatchOptProgress] = useState({ current: 0, total: 0 })

  // Modals for optimize question click + auth/subscription gating
  const [optimizeConfirmQ, setOptimizeConfirmQ] = useState<number | null>(null) // question index for confirmation
  const [readOnlyOptModal, setReadOnlyOptModal] = useState<string | null>(null) // brand name for "not ready" modal
  const [subscriptionModal, setSubscriptionModal] = useState(false)
  const [apiKeyModal, setApiKeyModal] = useState(false)
  const [loginModal, setLoginModal] = useState(false)

  // Brand Intelligence state
  const [brandProfile, setBrandProfile] = useState<BrandProfile | null>(null)
  const [brandList, setBrandList] = useState<BrandProfileSummary[]>([])
  const [, setBrandLoading] = useState(false)
  const [brandSaving, setBrandSaving] = useState(false)
  const [brandDomain, setBrandDomain] = useState('')
  const [brandEditing, setBrandEditing] = useState(false)
  // Brand form state (editable fields)
  const [brandForm, setBrandForm] = useState({
    brand_name: '',
    description: '',
    categories: '' as string,
    products: '' as string,
    primary_audience: '',
    key_use_cases: '' as string,
    differentiators: '' as string,
  })
  const [brandCompetitors, setBrandCompetitors] = useState<BrandCompetitor[]>([])
  const [brandQueries, setBrandQueries] = useState<TargetQuery[]>([])
  const [brandMessages, setBrandMessages] = useState<KeyMessage[]>([])
  const [brandPresenceForm, setBrandPresenceForm] = useState({
    youtube_url: '', subreddits: '', review_site_urls: '', social_profiles: '', content_assets: '', podcasts: '',
  })
  // Brand auto-discovery state
  const [discovering, setDiscovering] = useState(false)
  const [discoverMessages, setDiscoverMessages] = useState<string[]>([])
  const [discoveredCompetitors, setDiscoveredCompetitors] = useState<DiscoveredCompetitor[]>([])
  const [discoverSelected, setDiscoverSelected] = useState<Set<number>>(new Set())
  const [suggestingQueries, setSuggestingQueries] = useState(false)
  const [suggestMessages, setSuggestMessages] = useState<string[]>([])
  const [suggestedQueries, setSuggestedQueries] = useState<SuggestedQuery[]>([])
  const [suggestSelected, setSuggestSelected] = useState<Set<number>>(new Set())
  const [generatingDesc, setGeneratingDesc] = useState(false)
  const [generateMessages, setGenerateMessages] = useState<string[]>([])
  const [predictingAudience, setPredictingAudience] = useState(false)
  const [predictAudienceMessages, setPredictAudienceMessages] = useState<string[]>([])
  const [suggestingClaims, setSuggestingClaims] = useState(false)
  const [suggestClaimMessages, setSuggestClaimMessages] = useState<string[]>([])
  const [suggestedClaims, setSuggestedClaims] = useState<SuggestedClaim[]>([])
  const [suggestClaimSelected, setSuggestClaimSelected] = useState<Set<number>>(new Set())
  const [predictingDiffs, setPredictingDiffs] = useState(false)
  const [predictDiffMessages, setPredictDiffMessages] = useState<string[]>([])
  const [suggestedDiffs, setSuggestedDiffs] = useState<SuggestedDifferentiator[]>([])
  const [suggestDiffSelected, setSuggestDiffSelected] = useState<Set<number>>(new Set())
  const [brandPresenceComplete, setBrandPresenceComplete] = useState(false)
  const [brandSections, setBrandSections] = useState({ audience: false, competitors: true, queries: false, presence: false })
  const [quickSetupRunning, setQuickSetupRunning] = useState(false)
  const [quickSetupStep, setQuickSetupStep] = useState('')

  const [confirmDeleteBrand, setConfirmDeleteBrand] = useState(false)
  const [confirmDeleteAnalysis, setConfirmDeleteAnalysis] = useState<string | null>(null)
  const [confirmDeleteOptimization, setConfirmDeleteOptimization] = useState<string | null>(null)

  // Video Authority state
  const [videoView, setVideoView] = useState<VideoView>('input')
  const [videoChannelURL, setVideoChannelURL] = useState('')
  const [videoURLs, setVideoURLs] = useState<string[]>([''])
  const [videoBrandURL, setVideoBrandURL] = useState('')
  const [videoSearchTerms, setVideoSearchTerms] = useState<string[]>([])
  const [videoSearchTermSources, setVideoSearchTermSources] = useState<Map<string, 'brand' | 'optimization' | 'user'>>(new Map())
  const [videoSearchTermInput, setVideoSearchTermInput] = useState('')
  const [videoIncludeBrandSearches, setVideoIncludeBrandSearches] = useState(true)
  const [videoDiscovering, setVideoDiscovering] = useState(false)
  const [discoveredVideos, setDiscoveredVideos] = useState<YouTubeVideo[]>([])
  const [selectedVideoIds, setSelectedVideoIds] = useState<Set<string>>(new Set())
  const [videoQuotaEstimate, setVideoQuotaEstimate] = useState(0)
  const [videoAnalyzing, setVideoAnalyzing] = useState(false)
  const [videoAutoArchive, setVideoAutoArchive] = useState(true)
  const videoAbortRef = useRef<AbortController | null>(null)
  const [videoMessages, setVideoMessages] = useState<string[]>([])
  const [videoAnalysis, setVideoAnalysis] = useState<VideoAnalysis | null>(null)
  const [videoAnalysisList, setVideoAnalysisList] = useState<VideoAnalysisSummary[]>([])
  // videoResultTab removed — unified report has no tabs
  const [videoDomain, setVideoDomain] = useState('')
  const [confirmDeleteVideoAnalysis, setConfirmDeleteVideoAnalysis] = useState(false)
  const [videoExpandedCards, setVideoExpandedCards] = useState<Set<string>>(new Set())
  const [videoDetails, setVideoDetails] = useState<VideoDetail[]>([])
  const [videoDetailsExpanded, setVideoDetailsExpanded] = useState<Set<string>>(new Set())
  const [videoSettingsOpen, setVideoSettingsOpen] = useState(false)
  // Video review filters
  const [videoFilterTags, setVideoFilterTags] = useState<Set<string>>(new Set(['own', 'direct_mention', 'competitor_comparison', 'category_content']))
  const [videoFilterMinViews, setVideoFilterMinViews] = useState(0)
  const [videoFilterRecency, setVideoFilterRecency] = useState<'30d' | '90d' | '1y' | '3y' | 'all'>('1y')

  // Reddit Authority state
  const [redditView, setRedditView] = useState<RedditView>('input')
  const [redditSubreddits, setRedditSubreddits] = useState<string[]>([])
  const [redditSubredditInput, setRedditSubredditInput] = useState('')
  const [redditSearchTerms, setRedditSearchTerms] = useState<string[]>([])
  const [redditSearchTermSources, setRedditSearchTermSources] = useState<Map<string, 'brand' | 'optimization'>>(new Map())
  const [redditSearchTermInput, setRedditSearchTermInput] = useState('')
  const [redditDiscovering, setRedditDiscovering] = useState(false)
  const [discoveredThreads, setDiscoveredThreads] = useState<RedditThreadSummary[]>([])
  const [discoveredSubreddits, setDiscoveredSubreddits] = useState<string[]>([])
  const [selectedThreadIds, setSelectedThreadIds] = useState<Set<string>>(new Set())
  const [redditAnalyzing, setRedditAnalyzing] = useState(false)
  const redditAbortRef = useRef<AbortController | null>(null)
  const [redditMessages, setRedditMessages] = useState<string[]>([])
  const [redditAnalysis, setRedditAnalysis] = useState<RedditAnalysis | null>(null)
  const [redditAnalysisList, setRedditAnalysisList] = useState<RedditAnalysisSummary[]>([])
  const [redditDomain, setRedditDomain] = useState('')
  const [confirmDeleteRedditAnalysis, setConfirmDeleteRedditAnalysis] = useState(false)
  const [redditSettingsOpen, setRedditSettingsOpen] = useState(false)
  const [redditTimeFilter, setRedditTimeFilter] = useState<'month' | 'year' | 'all'>('year')
  const [redditAutoArchive, setRedditAutoArchive] = useState(true)

  // Search Visibility state
  const [searchAnalyzing, setSearchAnalyzing] = useState(false)
  const searchAbortRef = useRef<AbortController | null>(null)
  const [searchMessages, setSearchMessages] = useState<string[]>([])
  const [searchAnalysis, setSearchAnalysis] = useState<SearchAnalysis | null>(null)
  const [searchAnalysisList, setSearchAnalysisList] = useState<SearchAnalysisSummary[]>([])
  const [confirmDeleteSearchAnalysis, setConfirmDeleteSearchAnalysis] = useState(false)
  const [searchAutoArchive, setSearchAutoArchive] = useState(true)

  // Failed Analyses
  const [failedAnalyses, setFailedAnalyses] = useState<FailedAnalysis[]>([])

  // LLM Test state
  const [testView, setTestView] = useState<'input' | 'results'>('input')
  const [testQueries, setTestQueries] = useState<TestQuery[]>([])
  const [testSelectedProviders, setTestSelectedProviders] = useState<string[]>([])
  const [testAvailableProviders, setTestAvailableProviders] = useState<{ provider: string; status: string; preferred_model?: string }[]>([])
  const [testResults, setTestResults] = useState<LLMTestResult | null>(null)
  const [testAnalyzing, setTestAnalyzing] = useState(false)
  const [testMessages, setTestMessages] = useState<string[]>([])
  const [testError, setTestError] = useState('')
  const [confirmDeleteTest, setConfirmDeleteTest] = useState(false)
  const [testHistory, setTestHistory] = useState<LLMTestResult[]>([])
  const [testProviderModels, setTestProviderModels] = useState<{ id: string; name: string; models: { id: string; name: string }[] }[]>([])
  const [testModelSelections, setTestModelSelections] = useState<Record<string, string>>({})
  const [competitorTests, setCompetitorTests] = useState<LLMTestResult[]>([])
  const [showCompetitorOverlay, setShowCompetitorOverlay] = useState(false)
  const [competitorDomain, setCompetitorDomain] = useState('')
  const [showCompetitorModal, setShowCompetitorModal] = useState(false)
  const [testCompareRun, setTestCompareRun] = useState<LLMTestResult | null>(null)
  const [dismissedInsights, setDismissedInsights] = useState<Set<string>>(new Set())
  const testAbortRef = useRef<AbortController | null>(null)
  const testMessagesEndRef = useRef<HTMLDivElement>(null)
  const [testExpandedCells, setTestExpandedCells] = useState<Set<string>>(new Set())
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set())
  const [visibilityScore, setVisibilityScore] = useState<{ score: number; components: { name: string; score: number; weight: number; available: boolean }[]; available: number } | null>(null)
  const [visibilityScoreVersion, setVisibilityScoreVersion] = useState(0)

  // Auto-scroll refs for SSE message containers
  const videoMessagesEndRef = useRef<HTMLDivElement>(null)
  const redditMessagesEndRef = useRef<HTMLDivElement>(null)
  const searchMessagesEndRef = useRef<HTMLDivElement>(null)

  // PDF Report state
  const [pdfGenerating, setPdfGenerating] = useState(false)
  const [pdfProgress, setPdfProgress] = useState('')
  const pdfAbortRef = useRef<AbortController | null>(null)

  // SaaS auth state
  const [configLoaded, setConfigLoaded] = useState(false)
  const [saasEnabled, setSaasEnabled] = useState(false)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [unreadCount, setUnreadCount] = useState(0)
  const [showCredits, setShowCredits] = useState(false)
  const [tenantCredits, setTenantCredits] = useState(0)
  const [hasActivePlan, setHasActivePlan] = useState(false)
  const [apiKeyStatus, setApiKeyStatus] = useState<'loading' | 'active' | 'invalid' | 'no_credits' | 'unconfigured'>('loading')
  const [hasYouTubeKey, setHasYouTubeKey] = useState(false)
  const [userTenantRole, setUserTenantRole] = useState<string>('')
  const [welcomeKeyModal, setWelcomeKeyModal] = useState(false)

  useEffect(() => {
    apiFetch('/api/config').then(r => r.ok ? r.json() : null).then(data => {
      if (data?.saas_enabled) {
        setSaasEnabled(true)
        fetchCurrentUser().then(setUser)
        fetchUnreadCount().then(setUnreadCount)
        // Fetch credits info
        apiFetch('/api/plans').then(r => r.ok ? r.json() : null).then(plans => {
          if (plans?.plans) {
            const hasCredits = plans.plans.some((p: { usageCreditsPerMonth: number; bonusCredits: number }) => p.usageCreditsPerMonth > 0 || p.bonusCredits > 0)
            setShowCredits(hasCredits)
            setTenantCredits((plans.tenantSubscriptionCredits || 0) + (plans.tenantPurchasedCredits || 0))
            setHasActivePlan(plans.billingStatus === 'active' || plans.billingWaived === true)
          }
        }).catch(() => {})
        // Fetch API key status
        apiFetch('/api/settings/api-keys/status').then(r => r.ok ? r.json() : null).then(data => {
          if (data?.status) setApiKeyStatus(data.status)
          if (data?.has_youtube_key) setHasYouTubeKey(true)
          if (data?.role) setUserTenantRole(data.role)
          // Show welcome modal once per session for owners who haven't set up API keys yet
          if (data?.role === 'owner' && data?.status === 'unconfigured' && !sessionStorage.getItem('llmopt_welcome_key_shown')) {
            sessionStorage.setItem('llmopt_welcome_key_shown', '1')
            setWelcomeKeyModal(true)
          }
        }).catch(() => {}).finally(() => setConfigLoaded(true))
      } else {
        setConfigLoaded(true)
      }
    }).catch(() => { setConfigLoaded(true) })
  }, [])

  // Re-fetch API key status when keys are updated in settings
  useEffect(() => {
    const handler = () => {
      apiFetch('/api/settings/api-keys/status').then(r => r.ok ? r.json() : null).then(data => {
        if (data?.status) setApiKeyStatus(data.status)
        setHasYouTubeKey(!!data?.has_youtube_key)
      }).catch(() => {})
    }
    window.addEventListener('apikey-updated', handler)
    return () => window.removeEventListener('apikey-updated', handler)
  }, [])

  // Telemetry: sync context + initial page view
  useEffect(() => { trackPageView() }, [])
  useEffect(() => { setTelemetryContext(activeTab, selectedDomain) }, [activeTab, selectedDomain])

  // Telemetry: tab switch tracking
  const prevTabRef = useRef(activeTab)
  useEffect(() => {
    if (prevTabRef.current !== activeTab) {
      trackEvent('custom.tab.switch', { from: prevTabRef.current, to: activeTab })
      prevTabRef.current = activeTab
    }
  }, [activeTab])

  // Telemetry: domain switch tracking
  const prevDomainRef = useRef(selectedDomain)
  useEffect(() => {
    if (prevDomainRef.current !== selectedDomain && selectedDomain) {
      trackEvent('custom.domain.switch', { from: prevDomainRef.current, to: selectedDomain })
      prevDomainRef.current = selectedDomain
    }
  }, [selectedDomain])

  // Brand dirty tracking — flag-based, set only on user/AI changes after load
  const brandDirtyRef = useRef(false)
  const brandLoadingRef = useRef(false)
  const [pendingNavAction, setPendingNavAction] = useState<(() => void) | null>(null)

  // Effect: detect state changes after loading. If brandLoadingRef is set, this is
  // from a load/reset (not user edits), so clear dirty. Otherwise mark dirty.
  useEffect(() => {
    if (!brandEditing) return
    if (brandLoadingRef.current) {
      brandLoadingRef.current = false
      brandDirtyRef.current = false
      return
    }
    brandDirtyRef.current = true
  }, [brandEditing, brandForm, brandCompetitors, brandQueries, brandMessages, brandPresenceForm, brandPresenceComplete])

  const isBrandDirty = useCallback(() => {
    return brandEditing && brandDirtyRef.current
  }, [brandEditing])

  // Navigation guard — wraps any action that would leave the brand editor
  // Skip in shared mode — brand is read-only, no unsaved changes possible
  const guardBrandNav = useCallback((action: () => void) => {
    if (sharedModeRef.current || !isBrandDirty()) {
      action()
    } else {
      setPendingNavAction(() => action)
    }
  }, [isBrandDirty])

  const fetchHistory = useCallback(async () => {
    try {
      const response = await apiFetch('/api/analyses')
      if (response.ok) {
        const data = await response.json()
        setHistory(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch history:', err)
    }
  }, [])

  const checkHealth = useCallback(async () => {
    setHealthChecking(true)
    try {
      const response = await apiFetch('/api/health/providers')
      if (response.ok) {
        const data = await response.json() as HealthCheck
        if (data.models && data.models.length > 0) {
          setHealthHistory(prev => [data, ...prev].slice(0, 50))
        }
      }
    } catch {
      // Network error — don't add fake entries
    } finally {
      setHealthChecking(false)
    }
  }, [])

  const fetchTodos = useCallback(async () => {
    setTodosLoading(true)
    try {
      const response = await apiFetch('/api/todos')
      if (response.ok) {
        const data = await response.json()
        setTodos(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch todos:', err)
    } finally {
      setTodosLoading(false)
    }
  }, [])

  const fetchOptList = useCallback(async () => {
    setOptListLoading(true)
    try {
      const response = await apiFetch('/api/optimizations')
      if (response.ok) {
        const data = await response.json()
        setOptList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch optimizations:', err)
    } finally {
      setOptListLoading(false)
    }
  }, [])

  // All unique domains across the app, sorted by most recent activity
  const allDomains = useMemo((): { domain: string; latestDate: string }[] => {
    // Keyed by domainKey() to deduplicate variations like https://x.com vs x.com
    const domainMap: Record<string, { display: string; latestDate: string }> = {}
    const track = (domain: string, date: string) => {
      const key = domainKey(domain)
      if (!key) return
      const existing = domainMap[key]
      if (!existing || new Date(date) > new Date(existing.latestDate)) {
        domainMap[key] = { display: domain, latestDate: date }
      }
    }
    for (const h of history) track(h.domain, h.created_at)
    for (const o of optList) track(o.domain, o.created_at)
    for (const b of brandList) track(b.domain, b.updated_at)
    for (const v of videoAnalysisList) track(v.domain, v.generated_at)
    for (const t of todos) track(t.domain, t.created_at)
    return Object.entries(domainMap)
      .map(([, v]) => ({ domain: v.display, latestDate: v.latestDate }))
      .sort((a, b) => new Date(b.latestDate).getTime() - new Date(a.latestDate).getTime())
  }, [history, optList, brandList, videoAnalysisList, todos])

  const loadSummaryForDomain = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/summary`)
      if (res.ok) {
        const data = await res.json()
        setActiveSummary(data.summary)
        setActiveSummaryStale(data.stale)
        return
      }
    } catch { /* ignore */ }
    setActiveSummary(null)
    setActiveSummaryStale(false)
  }, [])

  const deleteAnalysis = useCallback(async (id: string) => {
    try {
      const response = await apiFetch(`/api/analyses/${id}`, { method: 'DELETE' })
      if (response.ok) {
        fetchHistory()
        fetchOptList()
        fetchTodos()
        if (resultMeta?.id === id) {
          setResultMeta(null)
          setResult(null)
          setState('idle')
        }
      }
    } catch (err) {
      console.error('Failed to delete analysis:', err)
    }
    setConfirmDeleteAnalysis(null)
  }, [fetchHistory, fetchOptList, fetchTodos, resultMeta])

  const deleteOptimization = useCallback(async (id: string) => {
    try {
      const response = await apiFetch(`/api/optimizations/${id}`, { method: 'DELETE' })
      if (response.ok) {
        fetchOptList()
        fetchTodos()
        setSelectedOpt(null)
        setOptimizeView('list')
      }
    } catch (err) {
      console.error('Failed to delete optimization:', err)
    }
    setConfirmDeleteOptimization(null)
  }, [fetchOptList, fetchTodos])

  const loadOptimizationDetail = useCallback(async (id: string) => {
    // In shared mode, use cached full optimization data instead of authenticated API call
    if (sharedModeRef.current) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cached = sharedOptimizations.find((o: any) => o.id === id)
      if (cached) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const data = cached as any
        setSelectedOpt({
          id: data.id,
          analysis_id: data.analysis_id || '',
          question_index: data.question_index ?? -1,
          question: data.question || '',
          domain: data.domain || '',
          page_urls: data.page_urls || [],
          result: data.result || {},
          raw_text: '',
          model: data.model || '',
          public: data.public || false,
          brand_status: data.brand_status || '',
          brand_context_used: data.brand_context_used || false,
          brand_profile_updated_at: data.brand_profile_updated_at,
          created_at: data.created_at || '',
        } as FullOptimization)
        setOptimization(data.result)
        setOptimizationMeta({
          id: data.id,
          model: data.model || '',
          createdAt: data.created_at || '',
          cached: false,
          question: data.question || '',
          questionIndex: data.question_index ?? -1,
          brandStatus: data.brand_status || undefined,
          brandContextUsed: data.brand_context_used || false,
          brandProfileUpdatedAt: data.brand_profile_updated_at || undefined,
        })
        // Use todos from shared data for this optimization
        setOptTodos(todos.filter(t => t.optimization_id === id))
        setOptimizeView('detail')
      }
      return
    }
    try {
      const [optRes, todosRes] = await Promise.all([
        apiFetch(`/api/optimizations/${id}`),
        apiFetch(`/api/todos?optimization_id=${id}`),
      ])
      if (optRes.ok) {
        const data = await optRes.json() as FullOptimization
        setSelectedOpt(data)
        setOptimization(data.result)
        setOptimizationMeta({
          id: data.id,
          model: data.model,
          createdAt: data.created_at,
          cached: false,
          question: data.question,
          questionIndex: data.question_index,
          brandStatus: data.brand_status || undefined,
          brandContextUsed: data.brand_context_used || false,
          brandProfileUpdatedAt: data.brand_profile_updated_at || undefined,
        })
      }
      if (todosRes.ok) {
        const todosData = await todosRes.json()
        setOptTodos(todosData || [])
      }
      setOptimizeView('detail')
    } catch (err) {
      console.error('Failed to load optimization:', err)
    }
  }, [sharedOptimizations, todos])

  const fetchPopularDomains = useCallback(async () => {
    try {
      const response = await apiFetch('/api/share/popular')
      if (response.ok) {
        const data = await response.json()
        setPopularDomains(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch popular domains:', err)
    }
  }, [])

  const fetchDomainShare = useCallback(async (domain: string) => {
    setShareLoading(true)
    setShareCopied(false)
    try {
      const response = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/share`)
      if (response.ok) {
        const data = await response.json()
        setDomainShareState(data)
      }
    } catch (err) {
      console.error('Failed to fetch domain share:', err)
    } finally {
      setShareLoading(false)
    }
  }, [])

  const setDomainVisibility = useCallback(async (domain: string, visibility: string) => {
    trackEventImmediate('custom.share.visibility_change', { domain, visibility })
    setShareLoading(true)
    try {
      const response = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/share`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ visibility }),
      })
      if (response.ok) {
        const data = await response.json()
        setDomainShareState(data)
        setShareCopied(false)
        if (visibility === 'popular' || visibility === 'private') {
          fetchPopularDomains()
        }
      }
    } catch (err) {
      console.error('Failed to set domain visibility:', err)
    } finally {
      setShareLoading(false)
    }
  }, [fetchPopularDomains])

  const fetchHealthTimeline = useCallback(async (hours = 24) => {
    setHealthTimelineLoading(true)
    try {
      const response = await apiFetch(`/api/health/history?hours=${hours}`)
      if (response.ok) {
        const data = await response.json()
        setHealthTimeline(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch health timeline:', err)
    } finally {
      setHealthTimelineLoading(false)
    }
  }, [])

  // Brand Intelligence callbacks
  const emptyPresence: BrandPresence = { youtube_url: '', subreddits: [], review_site_urls: [], social_profiles: [], content_assets: [], podcasts: [] }

  const fetchBrandList = useCallback(async () => {
    setBrandLoading(true)
    try {
      const response = await apiFetch('/api/brands')
      if (response.ok) {
        const data = await response.json()
        setBrandList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch brands:', err)
    } finally {
      setBrandLoading(false)
    }
  }, [])

  const loadBrandProfile = useCallback(async (domain: string) => {
    setBrandLoading(true)
    try {
      const response = await apiFetch(`/api/brands/${encodeURIComponent(domain)}`)
      if (response.ok) {
        const data = await response.json() as BrandProfile
        setBrandProfile(data)
        setBrandDomain(data.domain)
        setBrandForm({
          brand_name: data.brand_name || '',
          description: data.description || '',
          categories: (data.categories || []).join(', '),
          products: (data.products || []).join(', '),
          primary_audience: data.primary_audience || '',
          key_use_cases: (data.key_use_cases || []).join(', '),
          differentiators: (data.differentiators || []).join(', '),
        })
        setBrandCompetitors(data.competitors || [])
        setBrandQueries(data.target_queries || [])
        setBrandMessages(data.key_messages || [])
        const p = data.presence || emptyPresence
        setBrandPresenceForm({
          youtube_url: p.youtube_url || '',
          subreddits: (p.subreddits || []).join(', '),
          review_site_urls: (p.review_site_urls || []).join(', '),
          social_profiles: (p.social_profiles || []).join(', '),
          content_assets: (p.content_assets || []).join(', '),
          podcasts: (p.podcasts || []).join(', '),
        })
        setBrandPresenceComplete(data.presence_complete || false)
        brandLoadingRef.current = true
        setBrandEditing(true)
        return true
      } else if (response.status === 404) {
        return false
      }
    } catch (err) {
      console.error('Failed to load brand:', err)
    } finally {
      setBrandLoading(false)
    }
    return false
  }, [])

  const saveBrandProfile = useCallback(async () => {
    if (!brandDomain.trim()) return
    setBrandSaving(true)
    try {
      const splitTrim = (s: string) => s.split(',').map(x => x.trim()).filter(Boolean)
      const body: Partial<BrandProfile> = {
        brand_name: brandForm.brand_name,
        description: brandForm.description,
        categories: splitTrim(brandForm.categories),
        products: splitTrim(brandForm.products),
        primary_audience: brandForm.primary_audience,
        key_use_cases: splitTrim(brandForm.key_use_cases),
        competitors: brandCompetitors,
        target_queries: brandQueries,
        key_messages: brandMessages,
        differentiators: splitTrim(brandForm.differentiators),
        presence: {
          youtube_url: brandPresenceForm.youtube_url,
          subreddits: splitTrim(brandPresenceForm.subreddits),
          review_site_urls: splitTrim(brandPresenceForm.review_site_urls),
          social_profiles: splitTrim(brandPresenceForm.social_profiles),
          content_assets: splitTrim(brandPresenceForm.content_assets),
          podcasts: splitTrim(brandPresenceForm.podcasts),
        },
        presence_complete: brandPresenceComplete,
      }
      const response = await apiFetch(`/api/brands/${encodeURIComponent(brandDomain.trim())}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (response.ok) {
        const saved = await response.json() as BrandProfile
        setBrandProfile(saved)
        trackEvent('custom.brand.save', { domain: brandDomain })
        fetchBrandList()
        brandDirtyRef.current = false
      }
    } catch (err) {
      console.error('Failed to save brand:', err)
    } finally {
      setBrandSaving(false)
    }
  }, [brandDomain, brandForm, brandCompetitors, brandQueries, brandMessages, brandPresenceForm, brandPresenceComplete, fetchBrandList])

  const startNewBrand = useCallback((domain?: string) => {
    brandLoadingRef.current = true
    setBrandProfile(null)
    setBrandDomain(domain || url || '')
    setBrandForm({ brand_name: '', description: '', categories: '', products: '', primary_audience: '', key_use_cases: '', differentiators: '' })
    setBrandCompetitors([])
    setBrandQueries([])
    setBrandMessages([])
    setBrandPresenceForm({ youtube_url: '', subreddits: '', review_site_urls: '', social_profiles: '', content_assets: '', podcasts: '' })
    setBrandPresenceComplete(false)
    setDiscoveredCompetitors([])
    setSuggestedQueries([])
    setSuggestedClaims([])
    setSuggestedDiffs([])
    setBrandEditing(true)
  }, [url])

  // SSE helper for brand streaming endpoints
  const brandSSE = useCallback(async (
    url: string,
    setMessages: React.Dispatch<React.SetStateAction<string[]>>,
    setRunning: React.Dispatch<React.SetStateAction<boolean>>,
    onDone: (result: string) => void,
  ) => {
    setMessages([])
    setRunning(true)
    try {
      const response = await apiFetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      })
      if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`)

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)
          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue
          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status') setMessages(prev => [...prev, parsed.message])
            else if (eventType === 'done') { onDone(parsed.result); setRunning(false); return }
            else if (eventType === 'error') { const prefix = parsed.upstream ? 'Upstream API issue' : 'Error'; setMessages(prev => [...prev, `${prefix}: ${parsed.message}`]); setRunning(false); fetchFailedAnalyses(); return }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      setMessages(prev => [...prev, 'Connection failed: ' + (err as Error).message])
    } finally {
      setRunning(false)
    }
  }, [])

  const generateSummary = useCallback(async (domain: string) => {
    setGeneratingSummary(true)
    trackEventImmediate('custom.report.summary.start', { domain })
    setSummaryMessages([])

    await brandSSE(
      `/api/domains/${encodeURIComponent(domain)}/summary`,
      setSummaryMessages,
      setGeneratingSummary,
      async (resultStr: string) => {
        try {
          const parsed = JSON.parse(resultStr) as DomainSummaryResult
          trackEvent('custom.report.summary.complete', { domain })
          const res = await apiFetch(`/api/domains/${encodeURIComponent(domain)}/summary`)
          if (res.ok) {
            const data = await res.json()
            setActiveSummary(data.summary)
            setActiveSummaryStale(false)
          } else {
            setActiveSummary({ id: '', domain, result: parsed, model: '', optimization_ids: [], report_count: 0, includes_analysis: false, includes_video: false, includes_reddit: false, generated_at: new Date().toISOString() })
            setActiveSummaryStale(false)
          }
        } catch {
          setActiveSummary(null)
        }
      },
    )
  }, [brandSSE])

  // ── Video Authority callbacks ──

  const fetchVideoAnalysisList = useCallback(async () => {
    try {
      const res = await apiFetch('/api/video/analyses')
      if (res.ok) {
        const data = await res.json()
        setVideoAnalysisList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch video analyses:', err)
    }
  }, [])

  const loadVideoAnalysis = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/video/analyses/${encodeURIComponent(domain)}`)
      if (res.ok) {
        const data = await res.json() as VideoAnalysis
        setVideoAnalysis(data)
        setVideoDomain(domain)
        setVideoView('results')
      }
    } catch (err) {
      console.error('Failed to load video analysis:', err)
    }
  }, [])

  const prepopulateFromBrand = useCallback((domain: string) => {
    setVideoDomain(domain)
    setVideoBrandURL(domain)

    // Collect optimization questions for this domain
    const domainOpts = optList.filter(o => domainKey(o.domain) === domainKey(domain))
    const optQuestions = [...new Set(domainOpts.map(o => o.question))]

    // Find brand profile
    const brand = brandList.find(b => b.domain === domain)
    if (brand) {
      // Load full profile for presence data
      apiFetch(`/api/brands/${encodeURIComponent(domain)}`)
        .then(res => res.ok ? res.json() : null)
        .then((profile: BrandProfile | null) => {
          if (profile) {
            if (profile.presence?.youtube_url) setVideoChannelURL(profile.presence.youtube_url)
            const brandTerms = profile.target_queries?.map(q => q.query) || []
            // Add optimization questions, deduplicating case-insensitively
            const brandLower = new Set(brandTerms.map(t => t.toLowerCase()))
            const uniqueOptQuestions = optQuestions.filter(q => !brandLower.has(q.toLowerCase()))
            // Apply user customizations: add saved additions, remove saved removals
            const additions = profile.video_search_additions || []
            const removals = new Set((profile.video_search_removals || []).map(t => t.toLowerCase()))
            const baseLower = new Set([...brandTerms, ...uniqueOptQuestions].map(t => t.toLowerCase()))
            const uniqueAdditions = additions.filter(t => !baseLower.has(t.toLowerCase()))
            const allTerms = [...brandTerms, ...uniqueOptQuestions, ...uniqueAdditions]
              .filter(t => !removals.has(t.toLowerCase()))
            setVideoSearchTerms(allTerms)
            setVideoSearchTermSources(new Map([
              ...brandTerms.filter(t => !removals.has(t.toLowerCase())).map(t => [t, 'brand'] as [string, 'brand' | 'optimization']),
              ...uniqueOptQuestions.filter(q => !removals.has(q.toLowerCase())).map(q => [q, 'optimization'] as [string, 'brand' | 'optimization']),
              ...uniqueAdditions.filter(t => !removals.has(t.toLowerCase())).map(t => [t, 'user'] as [string, 'brand' | 'optimization' | 'user']),
            ]))
            // Collapse settings since they're now populated
            setVideoSettingsOpen(false)
          }
        })
        .catch(() => {})
    } else if (optQuestions.length > 0) {
      // No brand profile — still populate from optimization questions
      setVideoSearchTerms(optQuestions)
      setVideoSearchTermSources(new Map(optQuestions.map(q => [q, 'optimization'] as [string, 'brand' | 'optimization'])))
    }
  }, [brandList, optList])

  // Optimization questions newer than the current video analysis
  const videoStaleOptimizations = useMemo(() => {
    if (!videoAnalysis?.generated_at || !videoDomain) return []
    const analysisDate = new Date(videoAnalysis.generated_at)
    const seen = new Set<string>()
    return optList
      .filter(o =>
        domainKey(o.domain) === domainKey(videoDomain) &&
        new Date(o.created_at) > analysisDate
      )
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      .filter(o => {
        const q = o.question.toLowerCase()
        if (seen.has(q)) return false
        seen.add(q)
        return true
      })
  }, [videoAnalysis, videoDomain, optList])

  const isVideoAnalysisStale = useCallback((generatedAt: string, domain: string) => {
    const analysisDate = new Date(generatedAt)
    return optList.some(o =>
      domainKey(o.domain) === domainKey(domain) &&
      new Date(o.created_at) > analysisDate
    )
  }, [optList])

  // Filtered video list for review view
  const filteredDiscoveredVideos = useMemo(() => {
    const now = Date.now()
    const recencyMs: Record<string, number> = {
      '30d': 30 * 86400000,
      '90d': 90 * 86400000,
      '1y': 365 * 86400000,
      '3y': 3 * 365 * 86400000,
    }
    return discoveredVideos.filter(v => {
      if (!videoFilterTags.has(v.relevance_tag)) return false
      if ((v.view_count || 0) < videoFilterMinViews) return false
      if (videoFilterRecency !== 'all') {
        const age = now - new Date(v.published_at).getTime()
        if (age > (recencyMs[videoFilterRecency] || Infinity)) return false
      }
      return true
    })
  }, [discoveredVideos, videoFilterTags, videoFilterMinViews, videoFilterRecency])

  // Auto-select all filtered videos when filters change
  useEffect(() => {
    if (filteredDiscoveredVideos.length > 0) {
      setSelectedVideoIds(new Set(filteredDiscoveredVideos.map(v => v.video_id)))
    }
  }, [filteredDiscoveredVideos])

  // Computed stats for filter UI
  const videoViewCountMax = useMemo(() => {
    if (discoveredVideos.length === 0) return 0
    return Math.max(...discoveredVideos.map(v => v.view_count || 0))
  }, [discoveredVideos])

  const videoTagCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const v of discoveredVideos) {
      counts[v.relevance_tag] = (counts[v.relevance_tag] || 0) + 1
    }
    return counts
  }, [discoveredVideos])

  const videoDiscover = useCallback(async () => {
    if (!videoDomain.trim()) return
    setVideoDiscovering(true)
    trackEventImmediate('custom.report.video_discover.start', { domain: videoDomain })
    setVideoMessages([])
    setDiscoveredVideos([])
    setSelectedVideoIds(new Set())
    setVideoFilterTags(new Set(['own', 'direct_mention', 'competitor_comparison', 'category_content']))
    setVideoFilterMinViews(0)
    setVideoFilterRecency('1y')
    setVideoView('discovering')

    try {
      // Gather competitor names from brand profile
      let competitors: string[] = []
      let brandName = videoDomain
      try {
        const brandRes = await apiFetch(`/api/brands/${encodeURIComponent(videoDomain.trim())}`)
        if (brandRes.ok) {
          const brand = await brandRes.json() as BrandProfile
          brandName = brand.brand_name || videoDomain
          competitors = brand.competitors?.map(c => c.name) || []
        }
      } catch { /* ignore */ }

      const res = await apiFetch('/api/video/discover', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: videoDomain.trim(),
          brand_name: brandName,
          channel_url: videoChannelURL.trim(),
          search_terms: videoSearchTerms,
          competitors,
          include_brand_searches: videoIncludeBrandSearches,
        }),
      })

      if (!res.ok) {
        // SSE error before stream starts — fall back to text
        const text = await res.text()
        setVideoMessages([text || 'Discovery failed'])
        setVideoView('input')
        setVideoDiscovering(false)
        return
      }

      // Parse SSE stream
      const reader = res.body?.getReader()
      if (!reader) {
        setVideoMessages(['Streaming not supported'])
        setVideoView('input')
        setVideoDiscovering(false)
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''
      let currentEvent = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ') && currentEvent) {
            try {
              const data = JSON.parse(line.slice(6))
              if (currentEvent === 'status') {
                setVideoMessages(prev => [...prev, data.message])
              } else if (currentEvent === 'done') {
                const videos = (data.videos || []) as YouTubeVideo[]
                setDiscoveredVideos(videos)
                setSelectedVideoIds(new Set(videos.map((v: YouTubeVideo) => v.video_id)))
                setVideoQuotaEstimate(data.quota_estimate || 0)
                trackEvent('custom.report.video_discover.complete', { domain: videoDomain, videoCount: videos.length })
                setVideoView('review')
              } else if (currentEvent === 'error') {
                setVideoMessages(prev => [...prev, data.message || 'Discovery failed'])
                setVideoView('input')
              }
            } catch { /* ignore malformed JSON */ }
            currentEvent = ''
          }
        }
      }
    } catch (err) {
      setVideoMessages(['Connection failed: ' + (err as Error).message])
      setVideoView('input')
    } finally {
      setVideoDiscovering(false)
    }
  }, [videoDomain, videoChannelURL, videoSearchTerms, videoIncludeBrandSearches])

  const videoAnalyze = useCallback(async () => {
    if (selectedVideoIds.size === 0) return
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }
    const controller = new AbortController()
    videoAbortRef.current = controller
    setVideoAnalyzing(true)
    trackEventImmediate('custom.report.video.start', { domain: videoDomain, videoCount: selectedVideoIds.size })
    setVideoMessages([])
    setVideoView('running')

    // Auto-archive incomplete video todos for this domain
    if (videoAutoArchive && videoDomain.trim()) {
      try {
        await apiFetch('/api/todos/archive', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'video', domain: videoDomain.trim() }),
        })
      } catch { /* best effort */ }
    }

    try {
      const response = await apiFetch('/api/video/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: videoDomain.trim(),
          config: {
            channel_url: videoChannelURL.trim(),
            video_urls: videoURLs.filter(u => u.trim()),
            brand_url: videoBrandURL.trim(),
            search_terms: videoSearchTerms,
          },
          selected_video_ids: [...selectedVideoIds],
        }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        setVideoMessages(['Analysis request failed'])
        setVideoAnalyzing(false)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status') {
              setVideoMessages(prev => [...prev, parsed.message])
            } else if (eventType === 'progress') {
              // Replace last message in-place (e.g. transcript extraction counter)
              setVideoMessages(prev => prev.length > 0 ? [...prev.slice(0, -1), parsed.message] : [parsed.message])
            } else if (eventType === 'done') {
              // Parse the nested result JSON
              try {
                const result = typeof parsed.result === 'string' ? JSON.parse(parsed.result) : parsed.result
                const analysis: VideoAnalysis = {
                  id: '',
                  domain: result.domain || videoDomain,
                  config: result.config || { channel_url: videoChannelURL, video_urls: [], brand_url: videoBrandURL, search_terms: videoSearchTerms },
                  videos: result.videos || [],
                  result: result.result,
                  model: result.model || '',
                  brand_context_used: result.brand_context_used || false,
                  generated_at: result.generated_at || new Date().toISOString(),
                }
                setVideoAnalysis(analysis)
                trackEvent('custom.report.video.complete', { domain: videoDomain })
                setVideoView('results')
              } catch {
                setVideoMessages(prev => [...prev, 'Failed to parse analysis results'])
              }
              setVideoAnalyzing(false)
              fetchVideoAnalysisList()
              fetchTodos()
              setVisibilityScoreVersion(v => v + 1)
              return
            } else if (eventType === 'error') {
              const prefix = parsed.upstream ? 'Upstream API issue' : 'Error'
              setVideoMessages(prev => [...prev, `${prefix}: ${parsed.message}`])
              setVideoAnalyzing(false)
              fetchFailedAnalyses()
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setVideoMessages(prev => [...prev, 'Connection failed: ' + (err as Error).message])
      }
    } finally {
      setVideoAnalyzing(false)
      videoAbortRef.current = null
    }
  }, [selectedVideoIds, videoDomain, videoChannelURL, videoURLs, videoBrandURL, videoSearchTerms, videoAutoArchive, fetchVideoAnalysisList])

  const videoAnalyzeStop = useCallback(() => {
    videoAbortRef.current?.abort()
    setVideoAnalyzing(false)
    setVideoMessages(prev => [...prev, 'Analysis stopped by user'])
  }, [])

  const deleteVideoAnalysis = useCallback(async () => {
    if (!videoDomain) return
    try {
      await apiFetch(`/api/video/analyses/${encodeURIComponent(videoDomain)}`, { method: 'DELETE' })
      setVideoAnalysis(null)
      setVideoView('input')
      fetchVideoAnalysisList()
    } catch (err) {
      console.error('Failed to delete video analysis:', err)
    }
    setConfirmDeleteVideoAnalysis(false)
  }, [videoDomain, fetchVideoAnalysisList])

  // Fetch video analyses list on tab switch
  useEffect(() => {
    if (activeTab === 'video') {
      fetchVideoAnalysisList()
    }
  }, [activeTab, fetchVideoAnalysisList])

  // ── Reddit Authority functions ─────────────────────────────

  const fetchRedditAnalysisList = useCallback(async () => {
    try {
      const res = await apiFetch('/api/reddit/analyses')
      if (res.ok) {
        const data = await res.json()
        setRedditAnalysisList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch reddit analyses:', err)
    }
  }, [])

  const loadRedditAnalysis = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/reddit/analyses/${encodeURIComponent(domain)}`)
      if (res.ok) {
        const data = await res.json() as RedditAnalysis
        setRedditAnalysis(data)
        setRedditDomain(domain)
        setRedditView('results')
      }
    } catch (err) {
      console.error('Failed to load reddit analysis:', err)
    }
  }, [])

  const prepopulateRedditFromBrand = useCallback((domain: string) => {
    setRedditDomain(domain)

    // Collect optimization questions for this domain
    const domainOpts = optList.filter(o => domainKey(o.domain) === domainKey(domain))
    const optQuestions = [...new Set(domainOpts.map(o => o.question))]

    // Find brand profile
    const brand = brandList.find(b => b.domain === domain)
    if (brand) {
      apiFetch(`/api/brands/${encodeURIComponent(domain)}`)
        .then(res => res.ok ? res.json() : null)
        .then((profile: BrandProfile | null) => {
          if (profile) {
            // Populate subreddits from brand intelligence
            if (profile.presence?.subreddits?.length > 0) {
              setRedditSubreddits(profile.presence.subreddits)
            }
            const brandTerms = profile.target_queries?.map(q => q.query) || []
            const brandLower = new Set(brandTerms.map(t => t.toLowerCase()))
            const uniqueOptQuestions = optQuestions.filter(q => !brandLower.has(q.toLowerCase()))
            setRedditSearchTerms([...brandTerms, ...uniqueOptQuestions])
            setRedditSearchTermSources(new Map([
              ...brandTerms.map(t => [t, 'brand'] as [string, 'brand' | 'optimization']),
              ...uniqueOptQuestions.map(q => [q, 'optimization'] as [string, 'brand' | 'optimization']),
            ]))
            setRedditSettingsOpen(false)
          }
        })
        .catch(() => {})
    } else if (optQuestions.length > 0) {
      setRedditSearchTerms(optQuestions)
      setRedditSearchTermSources(new Map(optQuestions.map(q => [q, 'optimization'] as [string, 'brand' | 'optimization'])))
    }
  }, [brandList, optList])

  // Optimization questions newer than the current reddit analysis
  const redditStaleOptimizations = useMemo(() => {
    if (!redditAnalysis?.generated_at || !redditDomain) return []
    const analysisDate = new Date(redditAnalysis.generated_at)
    const seen = new Set<string>()
    return optList
      .filter(o =>
        domainKey(o.domain) === domainKey(redditDomain) &&
        new Date(o.created_at) > analysisDate
      )
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      .filter(o => {
        const q = o.question.toLowerCase()
        if (seen.has(q)) return false
        seen.add(q)
        return true
      })
  }, [redditAnalysis, redditDomain, optList])

  const isRedditAnalysisStale = useCallback((generatedAt: string, domain: string) => {
    const analysisDate = new Date(generatedAt)
    return optList.some(o =>
      domainKey(o.domain) === domainKey(domain) &&
      new Date(o.created_at) > analysisDate
    )
  }, [optList])

  // Search stale detection: newer optimizations since analysis
  const searchStaleOptimizations = useMemo(() => {
    if (!searchAnalysis?.generated_at || !selectedDomain) return []
    const analysisDate = new Date(searchAnalysis.generated_at)
    const seen = new Set<string>()
    return optList
      .filter(o =>
        domainKey(o.domain) === domainKey(selectedDomain) &&
        new Date(o.created_at) > analysisDate
      )
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      .filter(o => {
        const q = o.question.toLowerCase()
        if (seen.has(q)) return false
        seen.add(q)
        return true
      })
  }, [searchAnalysis, selectedDomain, optList])

  // Cross-report insights: data-driven suggestions connecting tabs
  const crossInsights = useMemo(() => {
    const insights: { tab: string; message: string; cta: string; targetTab: string }[] = []
    if (!selectedDomain) return insights

    // After Optimize: suggest video if content authority is low
    if (activeSummary?.result?.average_score && activeSummary.result.average_score < 60) {
      if (!videoAnalysis) insights.push({ tab: 'optimize', message: 'Your overall LLM visibility score is low. YouTube content can strengthen your brand narrative in training data.', cta: 'Run Video Analysis', targetTab: 'video' })
      if (!redditAnalysis) insights.push({ tab: 'optimize', message: 'Community discussions on Reddit contribute to LLM training data. Analyzing your Reddit presence may reveal opportunities.', cta: 'Run Reddit Analysis', targetTab: 'reddit' })
    }

    // After Video: if score is low, suggest optimization
    if (videoAnalysis?.result && videoAnalysis.result.overall_score < 50) {
      insights.push({ tab: 'video', message: 'Video authority is below average. Optimizing your content for specific queries can improve how LLMs reference your brand.', cta: 'View Optimization Reports', targetTab: 'optimize' })
    }

    // After Reddit: if sentiment is mixed, suggest brand profile review
    if (redditAnalysis?.result && redditAnalysis.result.overall_score < 50) {
      insights.push({ tab: 'reddit', message: 'Reddit sentiment needs attention. Review your brand messaging to address community concerns.', cta: 'Review Brand Profile', targetTab: 'brand' })
    }

    // After Test: if overall score is low, suggest running optimizations
    if (testResults && testResults.overall_score < 50) {
      insights.push({ tab: 'test', message: `LLMs mention your brand in only ${testResults.provider_summaries.reduce((s, p) => s + p.mention_rate, 0) / Math.max(testResults.provider_summaries.length, 1)}% of queries. Optimization reports provide actionable recommendations.`, cta: 'View Optimizations', targetTab: 'optimize' })
    }

    // If search is missing but other analyses exist
    if (!searchAnalysis && (videoAnalysis || redditAnalysis)) {
      insights.push({ tab: 'video', message: 'Search visibility affects whether AI systems discover your content. Run a search analysis for a complete picture.', cta: 'Run Search Analysis', targetTab: 'search' })
    }

    return insights
  }, [selectedDomain, activeSummary, videoAnalysis, redditAnalysis, searchAnalysis, testResults])

  const redditDiscover = useCallback(async () => {
    if (!redditDomain.trim()) return
    setRedditDiscovering(true)
    setRedditMessages([])
    setDiscoveredThreads([])
    setSelectedThreadIds(new Set())
    setRedditView('discovering')

    try {
      let competitors: string[] = []
      let brandName = redditDomain
      try {
        const brandRes = await apiFetch(`/api/brands/${encodeURIComponent(redditDomain.trim())}`)
        if (brandRes.ok) {
          const brand = await brandRes.json() as BrandProfile
          brandName = brand.brand_name || redditDomain
          competitors = brand.competitors?.map(c => c.name) || []
        }
      } catch { /* ignore */ }

      const res = await apiFetch('/api/reddit/discover', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: redditDomain.trim(),
          brand_name: brandName,
          subreddits: redditSubreddits,
          search_terms: redditSearchTerms,
          competitors,
          time_filter: redditTimeFilter,
        }),
      })

      if (!res.ok) {
        const text = await res.text()
        setRedditMessages([text || 'Discovery failed'])
        setRedditView('input')
        setRedditDiscovering(false)
        return
      }

      const reader = res.body?.getReader()
      if (!reader) {
        setRedditMessages(['Streaming not supported'])
        setRedditView('input')
        setRedditDiscovering(false)
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''
      let currentEvent = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ') && currentEvent) {
            try {
              const data = JSON.parse(line.slice(6))
              if (currentEvent === 'status') {
                setRedditMessages(prev => [...prev, data.message])
              } else if (currentEvent === 'done') {
                const threads = (data.threads || []) as RedditThreadSummary[]
                setDiscoveredThreads(threads)
                setDiscoveredSubreddits((data.discovered_subreddits || []) as string[])
                setSelectedThreadIds(new Set(threads.map(t => t.id)))
                setRedditView('review')
              } else if (currentEvent === 'error') {
                setRedditMessages(prev => [...prev, data.message || 'Discovery failed'])
                setRedditView('input')
              }
            } catch { /* ignore malformed JSON */ }
            currentEvent = ''
          }
        }
      }
    } catch (err) {
      setRedditMessages(['Connection failed: ' + (err as Error).message])
      setRedditView('input')
    } finally {
      setRedditDiscovering(false)
    }
  }, [redditDomain, redditSubreddits, redditSearchTerms, redditTimeFilter])

  const redditAnalyze = useCallback(async () => {
    if (selectedThreadIds.size === 0) return
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }
    const controller = new AbortController()
    redditAbortRef.current = controller
    setRedditAnalyzing(true)
    trackEventImmediate('custom.report.reddit.start', { domain: redditDomain })
    setRedditMessages([])
    setRedditView('running')

    if (redditAutoArchive && redditDomain.trim()) {
      try {
        await apiFetch('/api/todos/archive', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'reddit', domain: redditDomain.trim() }),
        })
      } catch { /* best effort */ }
    }

    try {
      const response = await apiFetch('/api/reddit/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: redditDomain.trim(),
          config: {
            subreddits: redditSubreddits,
            search_terms: redditSearchTerms,
            brand_url: redditDomain.trim(),
            time_filter: redditTimeFilter,
          },
          selected_thread_ids: [...selectedThreadIds],
          threads: discoveredThreads.filter(t => selectedThreadIds.has(t.id)),
        }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        setRedditMessages(['Analysis request failed'])
        setRedditAnalyzing(false)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status') {
              setRedditMessages(prev => [...prev, parsed.message])
            } else if (eventType === 'progress') {
              setRedditMessages(prev => prev.length > 0 ? [...prev.slice(0, -1), parsed.message] : [parsed.message])
            } else if (eventType === 'done') {
              try {
                const result = typeof parsed.result === 'string' ? JSON.parse(parsed.result) : parsed.result
                const analysis: RedditAnalysis = {
                  id: '',
                  domain: result.domain || redditDomain,
                  config: result.config || { subreddits: redditSubreddits, search_terms: redditSearchTerms, brand_url: redditDomain, time_filter: redditTimeFilter },
                  threads: result.threads || [],
                  result: result.result,
                  model: result.model || '',
                  brand_context_used: result.brand_context_used || false,
                  generated_at: result.generated_at || new Date().toISOString(),
                }
                setRedditAnalysis(analysis)
                trackEvent('custom.report.reddit.complete', { domain: redditDomain })
                setRedditView('results')
              } catch {
                setRedditMessages(prev => [...prev, 'Failed to parse analysis results'])
              }
              setRedditAnalyzing(false)
              fetchRedditAnalysisList()
              fetchTodos()
              setVisibilityScoreVersion(v => v + 1)
              return
            } else if (eventType === 'error') {
              const prefix = parsed.upstream ? 'Upstream API issue' : 'Error'
              setRedditMessages(prev => [...prev, `${prefix}: ${parsed.message}`])
              setRedditAnalyzing(false)
              fetchFailedAnalyses()
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setRedditMessages(prev => [...prev, 'Connection failed: ' + (err as Error).message])
      }
    } finally {
      setRedditAnalyzing(false)
      redditAbortRef.current = null
    }
  }, [selectedThreadIds, redditDomain, redditSubreddits, redditSearchTerms, redditTimeFilter, discoveredThreads, fetchRedditAnalysisList, redditAutoArchive])

  const redditAnalyzeStop = useCallback(() => {
    redditAbortRef.current?.abort()
    setRedditAnalyzing(false)
    setRedditMessages(prev => [...prev, 'Analysis stopped by user'])
  }, [])

  const deleteRedditAnalysis = useCallback(async () => {
    if (!redditDomain) return
    try {
      await apiFetch(`/api/reddit/analyses/${encodeURIComponent(redditDomain)}`, { method: 'DELETE' })
      setRedditAnalysis(null)
      setRedditView('input')
      fetchRedditAnalysisList()
    } catch (err) {
      console.error('Failed to delete reddit analysis:', err)
    }
    setConfirmDeleteRedditAnalysis(false)
  }, [redditDomain, fetchRedditAnalysisList])

  // ── Search Visibility functions ─────────────────────────────

  const fetchSearchAnalysisList = useCallback(async () => {
    try {
      const res = await apiFetch('/api/search/analyses')
      if (res.ok) {
        const data = await res.json()
        setSearchAnalysisList(data || [])
      }
    } catch (err) {
      console.error('Failed to fetch search analyses:', err)
    }
  }, [])

  const fetchFailedAnalyses = useCallback(async (domain?: string) => {
    try {
      const url = domain ? `/api/failed-analyses?domain=${encodeURIComponent(domain)}` : '/api/failed-analyses'
      const res = await apiFetch(url)
      if (res.ok) {
        setFailedAnalyses(await res.json())
      }
    } catch (err) {
      console.error('Failed to fetch failed analyses:', err)
    }
  }, [])

  const dismissFailedAnalysis = useCallback(async (id: string) => {
    try {
      await apiFetch(`/api/failed-analyses/${id}`, { method: 'DELETE' })
      setFailedAnalyses(prev => prev.filter(f => f.id !== id))
    } catch (err) {
      console.error('Failed to dismiss failed analysis:', err)
    }
  }, [])

  const loadSearchAnalysis = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/search/analyses/${encodeURIComponent(domain)}`)
      if (res.ok) {
        const data = await res.json() as SearchAnalysis
        setSearchAnalysis(data)
      }
    } catch (err) {
      console.error('Failed to load search analysis:', err)
    }
  }, [])

  const searchAnalyze = useCallback(async () => {
    if (!selectedDomain) return
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }
    const controller = new AbortController()
    searchAbortRef.current = controller
    setSearchAnalyzing(true)
    trackEventImmediate('custom.report.search.start', { domain: selectedDomain })
    setSearchMessages([])

    if (searchAutoArchive) {
      try {
        await apiFetch('/api/todos/archive', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'search', domain: selectedDomain }),
        })
      } catch { /* best effort */ }
    }

    try {
      const response = await apiFetch('/api/search/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: selectedDomain }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        setSearchAnalyzing(false)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status' || eventType === 'progress') {
              setSearchMessages(prev => [...prev, parsed.message])
            } else if (eventType === 'done') {
              const analysis = JSON.parse(parsed.result) as SearchAnalysis
              setSearchAnalysis(analysis)
              trackEvent('custom.report.search.complete', { domain: selectedDomain })
              setSearchAnalyzing(false)
              fetchSearchAnalysisList()
              setVisibilityScoreVersion(v => v + 1)
              return
            } else if (eventType === 'error') {
              const prefix = parsed.upstream ? 'Upstream API issue' : 'Error'
              setSearchMessages(prev => [...prev, `${prefix}: ${parsed.message}`])
              setSearchAnalyzing(false)
              fetchFailedAnalyses()
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setSearchMessages(prev => [...prev, `Error: ${err instanceof Error ? err.message : 'Unknown error'}`])
      }
    } finally {
      setSearchAnalyzing(false)
      searchAbortRef.current = null
    }
  }, [selectedDomain, fetchSearchAnalysisList, searchAutoArchive, saasEnabled, user, apiKeyStatus])

  const searchAnalyzeStop = useCallback(() => {
    searchAbortRef.current?.abort()
    setSearchAnalyzing(false)
    setSearchMessages(prev => [...prev, 'Analysis stopped by user'])
  }, [])

  const deleteSearchAnalysis = useCallback(async () => {
    if (!selectedDomain) return
    try {
      await apiFetch(`/api/search/analyses/${encodeURIComponent(selectedDomain)}`, { method: 'DELETE' })
      setSearchAnalysis(null)
      fetchSearchAnalysisList()
    } catch (err) {
      console.error('Failed to delete search analysis:', err)
    }
    setConfirmDeleteSearchAnalysis(false)
  }, [selectedDomain, fetchSearchAnalysisList])

  // LLM Test functions
  const runLLMTest = useCallback(async (force = false) => {
    if (!selectedDomain || testSelectedProviders.length === 0 || testQueries.length === 0) return
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }
    const controller = new AbortController()
    testAbortRef.current = controller
    setTestAnalyzing(true)
    trackEventImmediate('custom.report.test.start', { domain: selectedDomain, providers: testSelectedProviders.length, queries: testQueries.length })
    setTestMessages([])
    setTestError('')

    try {
      const response = await apiFetch('/api/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: selectedDomain,
          providers: testSelectedProviders,
          queries: testQueries,
          models: Object.keys(testModelSelections).length > 0 ? testModelSelections : undefined,
          force,
        }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        setTestAnalyzing(false)
        setTestError('Failed to start test')
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status' || eventType === 'progress') {
              setTestMessages(prev => [...prev, parsed.message])
            } else if (eventType === 'done') {
              const result = JSON.parse(parsed.result) as LLMTestResult
              setTestResults(result)
              trackEvent('custom.report.test.complete', { domain: selectedDomain })
              setTestView('results')
              setTestAnalyzing(false)
              setVisibilityScoreVersion(v => v + 1)
              return
            } else if (eventType === 'error') {
              setTestError(parsed.message)
              setTestMessages(prev => [...prev, `Error: ${parsed.message}`])
              setTestAnalyzing(false)
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setTestError(err instanceof Error ? err.message : 'Unknown error')
      }
    } finally {
      setTestAnalyzing(false)
      testAbortRef.current = null
    }
  }, [selectedDomain, testSelectedProviders, testQueries, saasEnabled, user, apiKeyStatus])

  const stopLLMTest = useCallback(() => {
    testAbortRef.current?.abort()
    setTestAnalyzing(false)
    setTestMessages(prev => [...prev, 'Test stopped by user'])
  }, [])

  // Run competitor test: same queries/providers but for a competitor domain
  const runCompetitorTest = useCallback(async (compDomain: string) => {
    if (!selectedDomain || !testResults || testSelectedProviders.length === 0) return
    if (saasEnabled && user && apiKeyStatus !== 'active') { setApiKeyModal(true); return }
    const controller = new AbortController()
    testAbortRef.current = controller
    setTestAnalyzing(true)
    setTestMessages([`Testing competitor: ${compDomain}...`])
    setTestError('')
    setShowCompetitorModal(false)

    try {
      const response = await apiFetch('/api/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: compDomain,
          providers: testResults.provider_summaries.map(ps => ps.provider_id),
          queries: testResults.queries,
          models: Object.keys(testModelSelections).length > 0 ? testModelSelections : undefined,
          competitor_of: selectedDomain,
          force: false,
        }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) { setTestAnalyzing(false); setTestError('Failed to start competitor test'); return }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status' || eventType === 'progress') {
              setTestMessages(prev => [...prev, parsed.message])
            } else if (eventType === 'done') {
              const result = JSON.parse(parsed.result) as LLMTestResult
              setCompetitorTests(prev => {
                const filtered = prev.filter(c => c.domain !== result.domain)
                return [...filtered, result]
              })
              setShowCompetitorOverlay(true)
              setTestAnalyzing(false)
              setTestView('results')
              return
            } else if (eventType === 'error') {
              setTestError(parsed.message)
              setTestMessages(prev => [...prev, `Error: ${parsed.message}`])
              setTestAnalyzing(false)
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) setTestError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setTestAnalyzing(false)
      testAbortRef.current = null
    }
  }, [selectedDomain, testResults, testSelectedProviders, testModelSelections, saasEnabled, user, apiKeyStatus])

  const deleteLLMTest = useCallback(async () => {
    if (!selectedDomain) return
    try {
      await apiFetch(`/api/test/${encodeURIComponent(selectedDomain)}`, { method: 'DELETE' })
      setTestResults(null)
      setTestView('input')
    } catch (err) {
      console.error('Failed to delete test:', err)
    }
    setConfirmDeleteTest(false)
  }, [selectedDomain])

  // PDF Report generation
  const generatePDF = useCallback(async () => {
    if (!selectedDomain || pdfGenerating) return
    const controller = new AbortController()
    pdfAbortRef.current = controller
    setPdfGenerating(true)
    setPdfProgress('')

    try {
      const response = await apiFetch(`/api/domains/${encodeURIComponent(selectedDomain)}/report/pdf`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: selectedDomain }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        setPdfGenerating(false)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break
          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = '', data = ''
          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }
          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)
            if (eventType === 'status') {
              setPdfProgress(parsed.message)
            } else if (eventType === 'done') {
              const pdfId = parsed.pdf_id
              const downloadUrl = `/api/domains/${encodeURIComponent(selectedDomain)}/report/pdf/${pdfId}`
              const pdfRes = await apiFetch(downloadUrl)
              const blob = await pdfRes.blob()
              const url = window.URL.createObjectURL(blob)
              const a = document.createElement('a')
              a.href = url
              a.download = `${selectedDomain.replace(/^https?:\/\//, '')}-llm-report.pdf`
              document.body.appendChild(a)
              a.click()
              a.remove()
              window.URL.revokeObjectURL(url)
              setPdfGenerating(false)
              setPdfProgress('')
              return
            } else if (eventType === 'error') {
              setPdfGenerating(false)
              setPdfProgress('')
              return
            }
          } catch { /* skip malformed */ }
        }
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        console.error('PDF generation failed:', err)
      }
    } finally {
      setPdfGenerating(false)
      pdfAbortRef.current = null
    }
  }, [selectedDomain, pdfGenerating])

  // Fetch reddit/search analyses list on tab switch
  useEffect(() => {
    if (activeTab === 'reddit') {
      fetchRedditAnalysisList()
    }
    if (activeTab === 'search') {
      fetchSearchAnalysisList()
    }
  }, [activeTab, fetchRedditAnalysisList, fetchSearchAnalysisList])

  // Auto-prepopulate reddit from brand when switching to reddit tab
  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'reddit') {
      fetchRedditAnalysisList()
      if (selectedDomain) {
        prepopulateRedditFromBrand(selectedDomain)
        // Auto-load existing report if one exists for this domain
        const existing = redditAnalysisList.find(a => domainKey(a.domain) === domainKey(selectedDomain))
        if (existing) {
          loadRedditAnalysis(existing.domain)
        }
      }
    }
  }, [activeTab, selectedDomain, prepopulateRedditFromBrand, fetchRedditAnalysisList, loadRedditAnalysis, redditAnalysisList])

  // Auto-load search analysis when switching to search tab
  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'search') {
      fetchSearchAnalysisList()
      if (selectedDomain) {
        const existing = searchAnalysisList.find(a => domainKey(a.domain) === domainKey(selectedDomain))
        if (existing) {
          loadSearchAnalysis(existing.domain)
        }
      }
    }
  }, [activeTab, selectedDomain, fetchSearchAnalysisList, loadSearchAnalysis])

  // Auto-load test results when switching to test tab
  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'test' && selectedDomain) {
      // Fetch available providers
      apiFetch('/api/settings/api-keys').then(r => r.ok ? r.json() : null).then(data => {
        if (data?.keys) {
          const active = data.keys.filter((k: { status: string; provider: string }) => k.status === 'active' && k.provider !== 'youtube')
          setTestAvailableProviders(active)
          setTestSelectedProviders(active.map((k: { provider: string }) => k.provider))
        }
      }).catch(() => {})
      // Fetch provider models for model selection dropdowns
      apiFetch('/api/providers/models').then(r => r.ok ? r.json() : []).then(data => {
        if (Array.isArray(data)) setTestProviderModels(data)
      }).catch(() => {})
      // Fetch existing test results
      apiFetch(`/api/test/${encodeURIComponent(selectedDomain)}`).then(r => {
        if (r.ok) return r.json()
        return null
      }).then(data => {
        if (data && data.id) {
          setTestResults(data)
          setTestView('results')
          // Fetch history for trend display
          apiFetch(`/api/test/${encodeURIComponent(selectedDomain)}/history`).then(r => r.ok ? r.json() : []).then(hist => {
            if (Array.isArray(hist)) setTestHistory(hist)
          }).catch(() => {})
          // Fetch competitor test results
          apiFetch(`/api/test/${encodeURIComponent(selectedDomain)}/competitors`).then(r => r.ok ? r.json() : []).then(comp => {
            if (Array.isArray(comp)) setCompetitorTests(comp)
          }).catch(() => {})
        } else {
          setTestView('input')
          setTestHistory([])
          // Generate suggested queries
          apiFetch('/api/test/generate-queries', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ domain: selectedDomain }),
          }).then(r => r.ok ? r.json() : null).then(qData => {
            if (qData?.queries) setTestQueries(qData.queries)
          }).catch(() => {})
        }
      }).catch(() => { setTestView('input') })
    }
  }, [activeTab, selectedDomain])

  // Auto-scroll SSE message containers to bottom
  useEffect(() => { videoMessagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [videoMessages])
  useEffect(() => { redditMessagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [redditMessages])
  useEffect(() => { searchMessagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [searchMessages])
  useEffect(() => { testMessagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [testMessages])

  // Auto-load domain summary and visibility score when selectedDomain changes
  // In shared mode, these are pre-loaded from the share endpoint — skip authenticated fetches.
  useEffect(() => {
    if (sharedModeRef.current) return
    if (selectedDomain) {
      loadSummaryForDomain(selectedDomain)
      apiFetch(`/api/visibility-score/${encodeURIComponent(selectedDomain)}`).then(r => r.ok ? r.json() : null).then(data => {
        if (data) setVisibilityScore(data)
        else setVisibilityScore(null)
      }).catch(() => setVisibilityScore(null))
    } else {
      setActiveSummary(null)
      setActiveSummaryStale(false)
      setVisibilityScore(null)
    }
  }, [selectedDomain, loadSummaryForDomain, visibilityScoreVersion])

  const discoverCompetitors = useCallback(async () => {
    if (!brandDomain.trim()) return
    await saveBrandProfile()
    setDiscoveredCompetitors([])
    setDiscoverSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/discover-competitors`,
      setDiscoverMessages,
      setDiscovering,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const comps = (parsed.competitors || []) as DiscoveredCompetitor[]
          setDiscoveredCompetitors(comps)
          setDiscoverSelected(new Set(comps.map((_: DiscoveredCompetitor, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, saveBrandProfile])

  const addSelectedCompetitors = useCallback(() => {
    const newComps = discoveredCompetitors
      .filter((_, i) => discoverSelected.has(i))
      .map(c => ({ name: c.name, url: c.url, relationship: c.relationship, notes: c.notes || '' }))
    setBrandCompetitors(prev => {
      const existing = new Set(prev.map(c => c.name.toLowerCase()))
      return [...prev, ...newComps.filter(c => !existing.has(c.name.toLowerCase()))]
    })
    setDiscoveredCompetitors([])
  }, [discoveredCompetitors, discoverSelected])

  const suggestQueries = useCallback(async () => {
    if (!brandDomain.trim()) return
    await saveBrandProfile()
    setSuggestedQueries([])
    setSuggestSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/suggest-queries`,
      setSuggestMessages,
      setSuggestingQueries,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const queries = (parsed.queries || []) as SuggestedQuery[]
          setSuggestedQueries(queries)
          setSuggestSelected(new Set(queries.map((_: SuggestedQuery, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, saveBrandProfile])

  const addSelectedQueries = useCallback(() => {
    const newQueries = suggestedQueries
      .filter((_, i) => suggestSelected.has(i))
    setBrandQueries(prev => {
      const existing = new Set(prev.map(q => q.query.toLowerCase()))
      return [...prev, ...newQueries.filter(q => !existing.has(q.query.toLowerCase()))]
    })
    setSuggestedQueries([])
  }, [suggestedQueries, suggestSelected])

  const generateDescription = useCallback(async () => {
    if (!brandDomain.trim()) return
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/generate-description`,
      setGenerateMessages,
      setGeneratingDesc,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          if (parsed.description) setBrandForm(prev => ({ ...prev, description: parsed.description }))
          if (parsed.brand_name && !brandForm.brand_name) setBrandForm(prev => ({ ...prev, brand_name: parsed.brand_name }))
          if (parsed.categories?.length && !brandForm.categories) setBrandForm(prev => ({ ...prev, categories: parsed.categories.join(', ') }))
          if (parsed.products?.length && !brandForm.products) setBrandForm(prev => ({ ...prev, products: parsed.products.join(', ') }))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, brandForm.brand_name, brandForm.categories, brandForm.products])

  const predictAudience = useCallback(async () => {
    if (!brandDomain.trim()) return
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/predict-audience`,
      setPredictAudienceMessages,
      setPredictingAudience,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          if (parsed.primary_audience && !brandForm.primary_audience)
            setBrandForm(prev => ({ ...prev, primary_audience: parsed.primary_audience }))
          if (parsed.key_use_cases?.length && !brandForm.key_use_cases)
            setBrandForm(prev => ({ ...prev, key_use_cases: parsed.key_use_cases.join(', ') }))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, brandForm.primary_audience, brandForm.key_use_cases])

  const suggestClaims = useCallback(async () => {
    if (!brandDomain.trim()) return
    await saveBrandProfile()
    setSuggestedClaims([])
    setSuggestClaimSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/suggest-claims`,
      setSuggestClaimMessages,
      setSuggestingClaims,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const claims = (parsed.claims || []) as SuggestedClaim[]
          setSuggestedClaims(claims)
          setSuggestClaimSelected(new Set(claims.map((_: SuggestedClaim, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, saveBrandProfile])

  const addSelectedClaims = useCallback(() => {
    const newClaims = suggestedClaims
      .filter((_: SuggestedClaim, i: number) => suggestClaimSelected.has(i))
      .map((c: SuggestedClaim) => ({ claim: c.claim, evidence_url: c.evidence_url || '', priority: c.priority || 'medium' }))
    setBrandMessages(prev => {
      const existing = new Set(prev.map(m => m.claim.toLowerCase()))
      return [...prev, ...newClaims.filter(c => !existing.has(c.claim.toLowerCase()))]
    })
    setSuggestedClaims([])
  }, [suggestedClaims, suggestClaimSelected])

  const predictDifferentiators = useCallback(async () => {
    if (!brandDomain.trim()) return
    await saveBrandProfile()
    setSuggestedDiffs([])
    setSuggestDiffSelected(new Set())
    await brandSSE(
      `/api/brands/${encodeURIComponent(brandDomain.trim())}/predict-differentiators`,
      setPredictDiffMessages,
      setPredictingDiffs,
      (result) => {
        try {
          const parsed = JSON.parse(result)
          const raw = parsed.differentiators || []
          // Handle both string[] and legacy {differentiator, reasoning}[] formats
          const diffs: string[] = raw.map((d: string | { differentiator: string }) =>
            typeof d === 'string' ? d : d.differentiator
          ).filter(Boolean)
          setSuggestedDiffs(diffs)
          setSuggestDiffSelected(new Set(diffs.map((_: string, i: number) => i)))
        } catch { /* ignore */ }
      },
    )
  }, [brandDomain, brandSSE, saveBrandProfile])

  const addSelectedDifferentiators = useCallback(() => {
    const newDiffs = suggestedDiffs
      .filter((_: string, i: number) => suggestDiffSelected.has(i))
    const currentDiffs = brandForm.differentiators ? brandForm.differentiators.split(',').map(s => s.trim()).filter(Boolean) : []
    const existingLower = new Set(currentDiffs.map(d => d.toLowerCase()))
    const merged = [...currentDiffs, ...newDiffs.filter(d => !existingLower.has(d.toLowerCase()))]
    setBrandForm(prev => ({ ...prev, differentiators: merged.join(', ') }))
    setSuggestedDiffs([])
  }, [suggestedDiffs, suggestDiffSelected, brandForm.differentiators])

  // Quick Setup: chain brand discovery steps sequentially
  const quickSetupBrand = useCallback(async () => {
    if (!brandDomain.trim() || quickSetupRunning) return
    setQuickSetupRunning(true)
    try {
      // Step 1: Generate description (populates name, description, categories, products)
      setQuickSetupStep('Analyzing site and generating description...')
      await brandSSE(
        `/api/brands/${encodeURIComponent(brandDomain.trim())}/generate-description`,
        setGenerateMessages, setGeneratingDesc,
        (result) => {
          try {
            const parsed = JSON.parse(result)
            setBrandForm(prev => ({
              ...prev,
              ...(parsed.description ? { description: parsed.description } : {}),
              ...(parsed.brand_name && !prev.brand_name ? { brand_name: parsed.brand_name } : {}),
              ...(parsed.categories?.length && !prev.categories ? { categories: parsed.categories.join(', ') } : {}),
              ...(parsed.products?.length && !prev.products ? { products: parsed.products.join(', ') } : {}),
            }))
          } catch { /* ignore */ }
        },
      )

      // Step 2: Predict audience
      setQuickSetupStep('Predicting target audience...')
      await brandSSE(
        `/api/brands/${encodeURIComponent(brandDomain.trim())}/predict-audience`,
        setPredictAudienceMessages, setPredictingAudience,
        (result) => {
          try {
            const parsed = JSON.parse(result)
            setBrandForm(prev => ({
              ...prev,
              ...(parsed.primary_audience && !prev.primary_audience ? { primary_audience: parsed.primary_audience } : {}),
              ...(parsed.key_use_cases?.length && !prev.key_use_cases ? { key_use_cases: parsed.key_use_cases.join(', ') } : {}),
            }))
          } catch { /* ignore */ }
        },
      )

      // Step 3: Predict differentiators (auto-add)
      setQuickSetupStep('Identifying differentiators...')
      await brandSSE(
        `/api/brands/${encodeURIComponent(brandDomain.trim())}/predict-differentiators`,
        setPredictDiffMessages, setPredictingDiffs,
        (result) => {
          try {
            const parsed = JSON.parse(result)
            const raw = parsed.differentiators || []
            const diffs: string[] = raw.map((d: string | { differentiator: string }) =>
              typeof d === 'string' ? d : d.differentiator
            ).filter(Boolean)
            if (diffs.length) {
              setBrandForm(prev => {
                const current = prev.differentiators ? prev.differentiators.split(',').map(s => s.trim()).filter(Boolean) : []
                const existLower = new Set(current.map(d => d.toLowerCase()))
                const merged = [...current, ...diffs.filter(d => !existLower.has(d.toLowerCase()))]
                return { ...prev, differentiators: merged.join(', ') }
              })
            }
          } catch { /* ignore */ }
        },
      )

      // Step 4: Suggest claims (auto-add)
      setQuickSetupStep('Discovering brand claims...')
      await brandSSE(
        `/api/brands/${encodeURIComponent(brandDomain.trim())}/suggest-claims`,
        setSuggestClaimMessages, setSuggestingClaims,
        (result) => {
          try {
            const parsed = JSON.parse(result)
            const claims = (parsed.claims || []) as SuggestedClaim[]
            if (claims.length) {
              setBrandMessages(prev => {
                const existing = new Set(prev.map(m => m.claim.toLowerCase()))
                const newOnes = claims
                  .map(c => ({ claim: c.claim, evidence_url: c.evidence_url || '', priority: c.priority || 'medium' }))
                  .filter(c => !existing.has(c.claim.toLowerCase()))
                return [...prev, ...newOnes]
              })
            }
          } catch { /* ignore */ }
        },
      )

      setQuickSetupStep('Done! Review and save your profile.')
    } catch { /* ignore */ }
    setQuickSetupRunning(false)
  }, [brandDomain, quickSetupRunning, brandSSE])

  // Brand completeness score
  const brandCompleteness = (() => {
    let score = 0
    // Core (30%): name, description, categories, products
    if (brandForm.brand_name) score += 8
    if (brandForm.description) score += 10
    if (brandForm.categories) score += 6
    if (brandForm.products) score += 6
    // Audience (15%): audience, use cases
    if (brandForm.primary_audience) score += 8
    if (brandForm.key_use_cases) score += 7
    // Competitors (25%)
    if (brandCompetitors.length > 0) score += 15
    if (brandCompetitors.length >= 3) score += 10
    // Queries/Voice (20%)
    if (brandQueries.length > 0) score += 8
    if (brandQueries.length >= 5) score += 4
    if (brandMessages.length > 0) score += 4
    if (brandForm.differentiators) score += 4
    // Presence (10%)
    if (brandPresenceComplete || brandPresenceForm.youtube_url || brandPresenceForm.subreddits || brandPresenceForm.review_site_urls) score += 10
    return Math.min(100, score)
  })()

  const updateTodoStatus = useCallback(async (id: string, status: 'todo' | 'completed' | 'backlogged' | 'archived') => {
    trackEvent('custom.todo.status_change', { status })
    try {
      const response = await apiFetch(`/api/todos/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status }),
      })
      if (response.ok) {
        const updater = (t: TodoItem) =>
          t.id === id ? {
            ...t, status,
            completed_at: status === 'completed' ? new Date().toISOString() : undefined,
            archived_at: status === 'archived' ? new Date().toISOString() : undefined,
          } : t
        setTodos(prev => prev.map(updater))
        setOptTodos(prev => prev.map(updater))
      }
    } catch (err) {
      console.error('Failed to update todo:', err)
    }
  }, [])

  // Tab-switch effects — split per tab to avoid cross-tab dependency cycles
  // In shared mode, data is pre-loaded from the share endpoint — skip authenticated fetches.
  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'todos') {
      fetchTodos()
      fetchBrandList()
      fetchOptList()
      fetchHistory()
    }
  }, [activeTab, selectedDomain, fetchTodos, fetchBrandList, fetchOptList, fetchHistory])

  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'optimize' && optimizeView === 'list') {
      fetchOptList()
    }
  }, [activeTab, optimizeView, fetchOptList])

  useEffect(() => {
    if (activeTab === 'status') {
      fetchHealthTimeline()
    }
  }, [activeTab, fetchHealthTimeline])

  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'brand' && !brandEditing && selectedDomain) {
      fetchBrandList()
      const existing = brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))
      if (existing) {
        loadBrandProfile(selectedDomain).then(found => {
          if (!found) {
            startNewBrand(selectedDomain)
            setTimeout(() => generateDescription(), 100)
          }
        })
      } else {
        startNewBrand(selectedDomain)
        // Auto-trigger description generation for new domains
        setTimeout(() => generateDescription(), 100)
      }
    }
  }, [activeTab, brandEditing, selectedDomain, fetchBrandList])

  useEffect(() => {
    if (sharedModeRef.current) return
    if (activeTab === 'video') {
      fetchVideoAnalysisList()
      if (selectedDomain) {
        // Always prepopulate settings from brand profile (channel URL, search terms)
        prepopulateFromBrand(selectedDomain)
        // Auto-load existing report if one exists for this domain
        const existing = videoAnalysisList.find(a => domainKey(a.domain) === domainKey(selectedDomain))
        if (existing) {
          loadVideoAnalysis(existing.domain)
        }
      }
    }
  }, [activeTab, selectedDomain, fetchVideoAnalysisList, prepopulateFromBrand])

  // Fetch all data on mount for domain dropdown (skip in shared mode)
  useEffect(() => {
    if (sharedModeRef.current) return
    fetchPopularDomains()
    fetchBrandList()
    fetchHistory()
    fetchOptList()
    fetchTodos()
    fetchVideoAnalysisList()
    fetchFailedAnalyses()
  }, [fetchPopularDomains, fetchBrandList, fetchHistory, fetchOptList, fetchTodos, fetchVideoAnalysisList, fetchFailedAnalyses])

  // Detect /share/{shareId} URL and load shared data
  useEffect(() => {
    const match = window.location.pathname.match(/^\/share\/([A-Za-z0-9]+)$/)
    if (!match) return
    const shareId = match[1]
    const shareParams = new URLSearchParams(window.location.search)
    const shareTab = shareParams.get('tab')
    const validShareTabs: ActiveTab[] = ['analyze', 'optimize', 'todos', 'brand', 'video']
    if (shareTab && validShareTabs.includes(shareTab as ActiveTab)) {
      setActiveTab(shareTab as ActiveTab)
    }
    setSharedMode(true)
    sharedModeRef.current = true
    setSharedShareId(shareId)
    setSharedLoading(true)
    trackPageView(window.location.href, 'Shared Domain')
    fetch(`/api/share/${shareId}`)
      .then(r => {
        if (!r.ok) { setSharedNotFound(true); setSharedLoading(false); return null }
        return r.json()
      })
      .then(data => {
        if (!data) return
        setSelectedDomain(data.domain)
        setUrl(data.domain)
        // Map analyses
        if (data.analyses?.length > 0) {
          const latest = data.analyses[0]
          setResult(latest.result)
          setResultMeta({ id: latest.id, model: latest.model, createdAt: latest.created_at, cached: false, brandContextUsed: latest.brand_context_used, brandProfileUpdatedAt: latest.brand_profile_updated_at || null })
          setState('done')
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          setHistory(data.analyses.map((a: any) => ({
            id: a.id, domain: a.domain,
            site_summary: a.result?.site_summary || a.result?.siteSummary || '',
            question_count: a.result?.questions?.length || 0,
            page_count: a.result?.crawled_pages?.length || a.result?.crawledPages?.length || 0,
            model: a.model, brand_context_used: a.brand_context_used,
            brand_profile_updated_at: a.brand_profile_updated_at,
            created_at: a.created_at,
          })))
        }
        // Map optimizations
        if (data.optimizations?.length > 0) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          setOptList(data.optimizations.map((o: any) => ({
            id: o.id, domain: o.domain, question: o.question,
            question_index: o.question_index ?? -1,
            overall_score: o.result?.overall_score ?? 0,
            model: o.model, public: false,
            brand_status: o.brand_status || '',
            brand_context_used: o.brand_context_used,
            brand_profile_updated_at: o.brand_profile_updated_at,
            created_at: o.created_at,
          })))
          // Store full optimization objects for detail view in shared mode
          setSharedOptimizations(data.optimizations)
        }
        // Map brand profile — populate form fields so Brand tab renders properly
        if (data.brand_profile) {
          const bp = data.brand_profile
          setBrandProfile(bp)
          setBrandDomain(data.domain)
          setBrandForm({
            brand_name: bp.brand_name || '',
            description: bp.description || '',
            categories: (bp.categories || []).join(', '),
            products: (bp.products || []).join(', '),
            primary_audience: bp.primary_audience || '',
            key_use_cases: (bp.key_use_cases || []).join(', '),
            differentiators: (bp.differentiators || []).join(', '),
          })
          setBrandCompetitors(bp.competitors || [])
          setBrandQueries(bp.target_queries || [])
          setBrandMessages(bp.key_messages || [])
          const p = bp.presence || { youtube_url: '', subreddits: [], review_site_urls: [], social_profiles: [], content_assets: [], podcasts: [] }
          setBrandPresenceForm({
            youtube_url: p.youtube_url || '',
            subreddits: (p.subreddits || []).join(', '),
            review_site_urls: (p.review_site_urls || []).join(', '),
            social_profiles: (p.social_profiles || []).join(', '),
            content_assets: (p.content_assets || []).join(', '),
            podcasts: (p.podcasts || []).join(', '),
          })
          setBrandPresenceComplete(bp.presence_complete || false)
          brandLoadingRef.current = true
          setBrandEditing(true)
        }
        // Map video analysis
        if (data.video_analysis) {
          setVideoAnalysis(data.video_analysis)
          setVideoDomain(data.domain)
          setVideoView('results')
        }
        // Map reddit analysis
        if (data.reddit_analysis) {
          setRedditAnalysis(data.reddit_analysis)
          setRedditDomain(data.domain)
          setRedditView('results')
        }
        // Map search analysis
        if (data.search_analysis) {
          setSearchAnalysis(data.search_analysis)
        }
        // Map LLM test results
        if (data.llm_test) {
          setTestResults(data.llm_test)
          setTestView('results')
        }
        // Map todos
        if (data.todos?.length > 0) {
          setTodos(data.todos)
        }
        // Map domain summary
        if (data.domain_summary) {
          setActiveSummary(data.domain_summary)
        }
        // Map visibility score
        if (data.visibility_score) {
          setVisibilityScore(data.visibility_score)
        }
        setSharedLoading(false)
      })
      .catch(() => { setSharedNotFound(true); setSharedLoading(false) })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Propagate selectedDomain to per-tab domain states
  useEffect(() => {
    if (!selectedDomain) return
    setBrandDomain(selectedDomain)
    setVideoDomain(selectedDomain)
    // In shared mode, don't reset views — share data load already set them
    if (!sharedModeRef.current) {
      setOptimizeView('list')
      setVideoView('input')
      setBrandEditing(false)
    }
  }, [selectedDomain])

  // Populate optScores from optList when analysis result or optList changes
  useEffect(() => {
    if (!result || !optList.length || !selectedDomain) {
      setOptScores({})
      return
    }
    const domainOpts = optList.filter(o => domainKey(o.domain) === domainKey(selectedDomain))
    const scores: Record<number, number> = {}
    // For each question in the analysis, find the latest optimization by question_index
    for (const opt of domainOpts) {
      if (opt.question_index >= 0 && opt.question_index < result.questions.length) {
        // Only keep the highest score (latest is first since list is sorted by date desc,
        // but question_index may have multiple runs — keep the latest/first)
        if (scores[opt.question_index] === undefined) {
          scores[opt.question_index] = opt.overall_score
        }
      }
    }
    setOptScores(scores)
  }, [result, optList, selectedDomain])

  // Clear selectedDomain if it no longer exists in allDomains
  // (but only if idle — don't clear during active analysis which adds new domains)
  // Skip in shared mode — domain is set from share data, not from allDomains
  useEffect(() => {
    if (sharedModeRef.current) return
    if (selectedDomain && allDomains.length > 0 && state !== 'analyzing' &&
        !allDomains.some(d => domainKey(d.domain) === domainKey(selectedDomain))) {
      setSelectedDomain('')
      setActiveTab('analyze')
    }
  }, [allDomains, selectedDomain, state])

  // Global health check polling — runs always, not just on Status tab
  useEffect(() => {
    checkHealth()
    const interval = setInterval(checkHealth, 5 * 60 * 1000)
    return () => clearInterval(interval)
  }, [checkHealth])


  const loadAnalysis = useCallback(async (id: string) => {
    try {
      const response = await apiFetch(`/api/analyses/${id}`)
      if (response.ok) {
        const data = await response.json()
        setResult(data.result)
        setResultMeta({
          id: data.id,
          model: data.model || '',
          createdAt: data.created_at || '',
          cached: false,
          brandContextUsed: data.brand_context_used || false,
          brandProfileUpdatedAt: data.brand_profile_updated_at || undefined,
        })
        setUrl(data.domain)
        setActiveTab('analyze')
        setState('done')
      }
    } catch (err) {
      console.error('Failed to load analysis:', err)
    }
  }, [])

  const loadBestAnalysis = useCallback(async (domain: string) => {
    try {
      const res = await apiFetch(`/api/analyses?domain=${encodeURIComponent(domainKey(domain))}`)
      if (!res.ok) return
      const list: AnalysisSummary[] = await res.json()
      // Backend sorts brand-intel reports first, then by date
      const best = list[0]
      if (best) loadAnalysis(best.id)
    } catch { /* ignore */ }
  }, [loadAnalysis])

  const analyze = useCallback(async (force?: boolean) => {
    const targetUrl = normalizeUrl(url)
    if (!targetUrl) return
    setUrl(targetUrl)

    // If the entered domain already exists in the dropdown, select it instead of re-analyzing
    const existingDomain = allDomains.find(d => domainKey(d.domain) === domainKey(targetUrl))
    if (existingDomain && !force && !forceAnalyze) {
      setSelectedDomain(existingDomain.domain)
      setUrl(existingDomain.domain)
      loadBestAnalysis(existingDomain.domain)
      return
    }

    // If the domain matches a popular brand, navigate to its shared view
    const popularMatch = popularDomains.find(pd => domainKey(pd.domain) === domainKey(targetUrl))
    if (popularMatch && !force && !forceAnalyze) {
      window.location.href = `/share/${popularMatch.share_id}`
      return
    }

    // SaaS gating: require login and active subscription for new analyses
    if (saasEnabled && !user) {
      window.location.href = '/login'
      return
    }
    if (saasEnabled && user && !hasActivePlan) {
      trackEventImmediate('custom.funnel.subscription_gate', { domain: targetUrl })
      setSubscriptionModal(true)
      return
    }
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }

    const shouldForce = force ?? forceAnalyze

    setState('analyzing')
    trackEventImmediate('custom.report.analyze.start', { domain: targetUrl, force: shouldForce })
    setStatusMessages([])
    setResult(null)
    setResultMeta(null)
    setError('')
    setForceAnalyze(false)

    const controller = new AbortController()
    abortRef.current = controller

    try {
      const response = await apiFetch('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: targetUrl, force: shouldForce }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        throw new Error(`HTTP ${response.status}`)
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let finished = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break

          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = ''
          let data = ''

          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }

          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)

            switch (eventType) {
              case 'status':
                setStatusMessages(prev => [...prev, parsed.message])
                break
              case 'text':
                break
              case 'done':
                try {
                  const analysisResult = JSON.parse(parsed.result) as AnalysisResult
                  setResult(analysisResult)
                  setResultMeta({
                    id: parsed.id || '',
                    model: parsed.model || '',
                    createdAt: parsed.created_at || '',
                    cached: parsed.cached || false,
                    brandContextUsed: parsed.brand_context_used || false,
                    brandProfileUpdatedAt: parsed.brand_profile_updated_at || undefined,
                  })
                  setState('done')
                  trackEvent('custom.report.analyze.complete', { domain: targetUrl, cached: parsed.cached })
                  setSelectedDomain(targetUrl)
                  fetchHistory()
                  // Load the best report for this domain (may be a better one with brand intel)
                  loadBestAnalysis(targetUrl)
                  finished = true
                } catch {
                  setError('Failed to parse analysis results')
                  setState('error')
                  finished = true
                }
                break
              case 'error':
                setError(parsed.message)
                setState('error')
                finished = true
                break
            }
          } catch {
            // Malformed SSE data, skip
          }
        }
      }

      if (!finished) {
        setError('Analysis stream ended unexpectedly')
        setState('error')
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setError('Connection failed: ' + (err as Error).message)
        setState('error')
      }
    }
  }, [url, forceAnalyze, fetchHistory, allDomains, loadBestAnalysis, saasEnabled, user, hasActivePlan, popularDomains])

  const cancel = useCallback(() => {
    abortRef.current?.abort()
    setState('idle')
  }, [])

  const optimizeQuestion = useCallback(async (questionIdx: number, force?: boolean) => {
    if (!resultMeta?.id) return
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }

    setOptimizing(true)
    trackEventImmediate('custom.report.optimize.start', { domain: url, questionIdx })
    setOptimizeMessages([])
    setOptimization(null)
    setOptimizationMeta(null)
    setOptimizeError('')
    setOptTodos([])
    setSelectedOpt(null)
    setOptimizeView('running')
    setActiveTab('optimize')

    // Auto-archive incomplete optimization todos for this question
    const questionText = result?.questions[questionIdx]?.question
    if (optimizeAutoArchive && url && questionText) {
      try {
        await apiFetch('/api/todos/archive', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source_type: 'optimization', domain: url, question: questionText }),
        })
      } catch { /* best effort */ }
    }

    const controller = new AbortController()
    optimizeAbortRef.current = controller

    try {
      const response = await apiFetch(`/api/analyses/${resultMeta.id}/questions/${questionIdx}/optimize`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ force: force || false }),
        signal: controller.signal,
      })

      if (!response.ok || !response.body) {
        throw new Error(`HTTP ${response.status}`)
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let finished = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        while (true) {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) break

          const message = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = ''
          let data = ''

          for (const line of message.split('\n')) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) data = line.slice(6)
          }

          if (!data || !eventType) continue

          try {
            const parsed = JSON.parse(data)

            switch (eventType) {
              case 'status':
                setOptimizeMessages(prev => [...prev, parsed.message])
                break
              case 'text':
                break
              case 'done':
                try {
                  const optResult = JSON.parse(parsed.result) as OptimizationResult
                  setOptimization(optResult)
                  setOptimizationMeta({
                    id: parsed.id || '',
                    model: parsed.model || '',
                    createdAt: parsed.created_at || '',
                    cached: parsed.cached || false,
                    question: result?.questions[questionIdx]?.question || '',
                    questionIndex: questionIdx,
                    brandStatus: parsed.brand_status || result?.questions[questionIdx]?.brand_status || undefined,
                    brandContextUsed: parsed.brand_context_used || false,
                    brandProfileUpdatedAt: parsed.brand_profile_updated_at || undefined,
                  })
                  setOptScores(prev => ({ ...prev, [questionIdx]: optResult.overall_score }))
                  trackEvent('custom.report.optimize.complete', { domain: url, score: optResult.overall_score, cached: parsed.cached })
                  setOptimizing(false)
                  setOptimizeView('detail')
                  setVisibilityScoreVersion(v => v + 1)
                  // Fetch todos after a short delay (they're created async)
                  if (parsed.id) {
                    setTimeout(async () => {
                      try {
                        const todosRes = await apiFetch(`/api/todos?optimization_id=${parsed.id}`)
                        if (todosRes.ok) {
                          const todosData = await todosRes.json()
                          setOptTodos(todosData || [])
                        }
                      } catch { /* ignore */ }
                    }, 1000)
                  }
                  finished = true
                } catch {
                  setOptimizeError('Failed to parse optimization results')
                  setOptimizing(false)
                  finished = true
                }
                break
              case 'error':
                setOptimizeError(parsed.upstream ? `Upstream API issue: ${parsed.message}` : parsed.message)
                setOptimizing(false)
                fetchFailedAnalyses()
                finished = true
                break
            }
          } catch {
            // Malformed SSE data, skip
          }
        }
      }

      if (!finished) {
        setOptimizeError('Optimization stream ended unexpectedly')
        setOptimizing(false)
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setOptimizeError('Connection failed: ' + (err as Error).message)
        setOptimizing(false)
      }
    }
  }, [resultMeta, result, optimizeAutoArchive, url])

  const cancelOptimize = useCallback(() => {
    optimizeAbortRef.current?.abort()
    setOptimizing(false)
    setOptimizeView('list')
  }, [])

  // Build category color map
  const categories = result?.questions
    ? [...new Set(result.questions.map(q => q.category))]
    : []
  const categoryColors: Record<string, string> = {}
  categories.forEach((cat, i) => {
    categoryColors[cat] = CATEGORY_COLORS[i % CATEGORY_COLORS.length]
  })

  const readOnly = sharedMode

  // Handle question click from Analyze tab — shows confirmation or jumps to existing report
  const handleQuestionClick = (questionIdx: number) => {
    if (readOnly) {
      // Check if optimization report exists for this question
      const existing = optList.find(o =>
        domainKey(o.domain) === domainKey(selectedDomain) && o.question_index === questionIdx
      )
      if (existing) {
        // Jump to existing report
        loadOptimizationDetail(existing.id)
        setActiveTab('optimize')
      } else {
        // Show "not ready" modal
        const brandName = brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.brand_name || selectedDomain
        setReadOnlyOptModal(brandName)
      }
      return
    }
    // Normal (authenticated) mode
    if (!saasEnabled || !user) {
      // Non-SaaS mode: just optimize directly
      optimizeQuestion(questionIdx)
      return
    }
    // Check if optimization report already exists
    const existing = optList.find(o =>
      domainKey(o.domain) === domainKey(selectedDomain) && o.question_index === questionIdx
    )
    if (existing) {
      // Jump directly to existing report
      loadOptimizationDetail(existing.id)
      setActiveTab('optimize')
      return
    }
    // Show confirmation modal
    setOptimizeConfirmQ(questionIdx)
  }

  // Batch optimize: run optimization for all un-optimized questions
  const batchOptimize = useCallback(async () => {
    if (!result || !resultMeta?.id) return
    if (saasEnabled && user && apiKeyStatus !== 'active') {
      setApiKeyModal(true)
      return
    }

    const optimized = new Set(optList.filter(o => domainKey(o.domain) === domainKey(selectedDomain)).map(o => o.question))
    const unoptimized = result.questions.map((q, i) => ({ q, i })).filter(({ q }) => !optimized.has(q.question))

    if (unoptimized.length === 0) return

    setBatchOptimizing(true)
    setBatchOptProgress({ current: 0, total: unoptimized.length })

    for (let idx = 0; idx < unoptimized.length; idx++) {
      const { i: questionIdx } = unoptimized[idx]
      setBatchOptProgress({ current: idx + 1, total: unoptimized.length })

      try {
        const response = await apiFetch(`/api/analyses/${resultMeta.id}/questions/${questionIdx}/optimize`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ force: false }),
        })
        if (!response.ok || !response.body) continue

        // Consume the SSE stream to completion
        const reader = response.body.getReader()
        while (true) {
          const { done } = await reader.read()
          if (done) break
        }
      } catch { /* continue with next question */ }
    }

    setBatchOptimizing(false)
    setVisibilityScoreVersion(v => v + 1)
    // Reload optimizations list
    apiFetch(`/api/optimizations?domain=${encodeURIComponent(selectedDomain)}`).then(r => r.ok ? r.json() : []).then(data => {
      if (Array.isArray(data)) setOptList(data)
    }).catch(() => {})
    // Reload todos
    apiFetch('/api/todos').then(r => r.ok ? r.json() : []).then(data => {
      if (Array.isArray(data)) setTodos(data)
    }).catch(() => {})
  }, [result, resultMeta, selectedDomain, optList, saasEnabled, user, apiKeyStatus])

  // Shared mode: loading / not-found states
  if (sharedLoading) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
          <p className="text-dark-400">Loading shared report...</p>
        </div>
      </div>
    )
  }

  if (sharedNotFound) {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center">
        <div className="text-center max-w-md mx-auto px-4">
          <div className="w-16 h-16 rounded-full bg-dark-800 flex items-center justify-center mx-auto mb-4">
            <svg className="w-8 h-8 text-dark-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" /></svg>
          </div>
          <h2 className="text-white text-xl font-semibold mb-2">Report Not Available</h2>
          <p className="text-dark-400 text-sm mb-6">This shared report is no longer available. The owner may have made it private.</p>
          <a href="/" className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-500 transition-colors text-sm font-medium">
            Go to LLM Optimizer
          </a>
        </div>
      </div>
    )
  }

  if (showDocs) {
    return (
      <div className="min-h-screen flex flex-col bg-dark-950">
        {/* Header */}
        <header className="sticky top-0 z-50 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800">
          <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <a href="/" className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                  <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
                  </svg>
                </div>
                <span className="text-lg font-bold text-white">LLM Optimizer</span>
              </a>
            </div>
            <button onClick={() => { if (isDocsURL) { window.location.href = '/' } else { setShowDocs(false) } }} className="text-sm text-dark-400 hover:text-white transition-colors flex items-center gap-1">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M10 19l-7-7m0 0l7-7m-7 7h18" /></svg>
              Back to App
            </button>
          </div>
        </header>

        {/* Docs Content */}
        <main className="flex-1 max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12 w-full">
          <h1 className="text-3xl font-bold text-white mb-2">API Documentation</h1>
          <p className="text-dark-400 mb-8">Programmatic access to your LLM Optimizer data. <a href="/api/v1/docs" target="_blank" rel="noopener noreferrer" className="text-primary-400 hover:text-primary-300">View raw markdown reference</a></p>

          {/* Authentication */}
          <section className="mb-12">
            <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" /></svg>
              Authentication
            </h2>
            <div className="bg-dark-900 border border-dark-700 rounded-xl p-6 space-y-4">
              <p className="text-dark-300">All API requests require a Bearer token in the <code className="text-primary-400 bg-dark-800 px-1.5 py-0.5 rounded text-sm">Authorization</code> header.</p>
              <div className="bg-dark-800 rounded-lg p-4">
                <p className="text-xs text-dark-500 mb-2">API Key (recommended for integrations)</p>
                <code className="text-sm text-green-400 font-mono">Authorization: Bearer lsk_your_api_key_here</code>
              </div>
              <p className="text-dark-400 text-sm"><strong className="text-dark-200">Admin keys</strong> auto-resolve to the root tenant. <strong className="text-dark-200">User keys</strong> require the <code className="text-primary-400 bg-dark-800 px-1 py-0.5 rounded text-xs">X-Tenant-ID</code> header.</p>
              <p className="text-dark-400 text-sm">Generate API keys from the admin panel under Settings &gt; API Keys.</p>
            </div>
          </section>

          {/* Rate Limits */}
          <section className="mb-12">
            <h2 className="text-xl font-semibold text-white mb-4">Rate Limits</h2>
            <div className="bg-dark-900 border border-dark-700 rounded-xl p-6">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div className="bg-dark-800 rounded-lg p-4 text-center">
                  <p className="text-2xl font-bold text-white">60</p>
                  <p className="text-dark-400">reads / minute</p>
                </div>
                <div className="bg-dark-800 rounded-lg p-4 text-center">
                  <p className="text-2xl font-bold text-white">30</p>
                  <p className="text-dark-400">writes / minute</p>
                </div>
              </div>
            </div>
          </section>

          {/* Endpoints */}
          <section className="mb-12">
            <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75L22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3l-4.5 16.5" /></svg>
              Endpoints
            </h2>

            {/* Domain Data */}
            <h3 className="text-lg font-medium text-dark-200 mt-8 mb-4">Domain Data (Read-only)</h3>
            <div className="space-y-3">
              {[
                { method: 'GET', path: '/api/v1/domains', desc: 'List all domains with data for your tenant' },
                { method: 'GET', path: '/api/v1/domains/{domain}/analysis', desc: 'Latest site analysis report' },
                { method: 'GET', path: '/api/v1/domains/{domain}/optimizations', desc: 'All answer optimization reports' },
                { method: 'GET', path: '/api/v1/domains/{domain}/video', desc: 'Video authority analysis (4-pillar)' },
                { method: 'GET', path: '/api/v1/domains/{domain}/reddit', desc: 'Reddit authority analysis (4-pillar)' },
                { method: 'GET', path: '/api/v1/domains/{domain}/search', desc: 'Search visibility analysis (5-pillar)' },
                { method: 'GET', path: '/api/v1/domains/{domain}/summary', desc: 'Cross-report executive summary' },
                { method: 'GET', path: '/api/v1/domains/{domain}/tests', desc: 'LLM knowledge test results (up to 10)' },
                { method: 'GET', path: '/api/v1/domains/{domain}/score', desc: 'Aggregate visibility score' },
                { method: 'GET', path: '/api/v1/domains/{domain}/brand', desc: 'Brand profile' },
              ].map((ep) => (
                <div key={ep.path} className="bg-dark-900 border border-dark-700 rounded-lg p-4 flex items-start gap-3">
                  <span className="bg-green-900/50 text-green-400 text-xs font-mono font-bold px-2 py-1 rounded shrink-0">{ep.method}</span>
                  <div>
                    <code className="text-sm text-white font-mono">{ep.path}</code>
                    <p className="text-dark-400 text-sm mt-1">{ep.desc}</p>
                  </div>
                </div>
              ))}
            </div>

            {/* Todos */}
            <h3 className="text-lg font-medium text-dark-200 mt-8 mb-4">Todos</h3>
            <div className="space-y-3">
              {[
                { method: 'GET', path: '/api/v1/todos', desc: 'List action items. Filter: ?status=todo&domain=example.com', color: 'green' },
                { method: 'GET', path: '/api/v1/todos/{id}', desc: 'Get a single todo by ID', color: 'green' },
                { method: 'PATCH', path: '/api/v1/todos/{id}', desc: 'Update todo status (admin/owner only). Body: {"status": "completed"}', color: 'yellow' },
                { method: 'POST', path: '/api/v1/todos/bulk-update', desc: 'Bulk update todo statuses (admin/owner, max 100). Body: {"ids": [...], "status": "archived"}', color: 'blue' },
              ].map((ep) => (
                <div key={ep.method + ep.path} className="bg-dark-900 border border-dark-700 rounded-lg p-4 flex items-start gap-3">
                  <span className={`${ep.color === 'green' ? 'bg-green-900/50 text-green-400' : ep.color === 'yellow' ? 'bg-yellow-900/50 text-yellow-400' : 'bg-blue-900/50 text-blue-400'} text-xs font-mono font-bold px-2 py-1 rounded shrink-0`}>{ep.method}</span>
                  <div>
                    <code className="text-sm text-white font-mono">{ep.path}</code>
                    <p className="text-dark-400 text-sm mt-1">{ep.desc}</p>
                  </div>
                </div>
              ))}
            </div>

            {/* Status values */}
            <div className="mt-6 bg-dark-900 border border-dark-700 rounded-xl p-6">
              <h4 className="text-sm font-medium text-dark-200 mb-3">Valid Todo Statuses</h4>
              <div className="flex flex-wrap gap-2">
                {['todo', 'completed', 'backlogged', 'archived'].map((s) => (
                  <code key={s} className="bg-dark-800 text-dark-300 px-2 py-1 rounded text-sm font-mono">{s}</code>
                ))}
              </div>
            </div>
          </section>

          {/* Example */}
          <section className="mb-12">
            <h2 className="text-xl font-semibold text-white mb-4">Quick Start</h2>
            <div className="bg-dark-900 border border-dark-700 rounded-xl overflow-hidden">
              <div className="bg-dark-800 px-4 py-2 text-xs text-dark-400 font-mono border-b border-dark-700">curl</div>
              <pre className="p-4 text-sm text-dark-300 font-mono overflow-x-auto whitespace-pre-wrap">
{`# List all domains
curl -H "Authorization: Bearer lsk_your_key" \\
  https://llmopt.fly.dev/api/v1/domains

# Get analysis for a domain
curl -H "Authorization: Bearer lsk_your_key" \\
  https://llmopt.fly.dev/api/v1/domains/example.com/analysis

# Get visibility score
curl -H "Authorization: Bearer lsk_your_key" \\
  https://llmopt.fly.dev/api/v1/domains/example.com/score

# Mark a todo as completed (admin/owner)
curl -X PATCH \\
  -H "Authorization: Bearer lsk_your_key" \\
  -H "Content-Type: application/json" \\
  -d '{"status": "completed"}' \\
  https://llmopt.fly.dev/api/v1/todos/TODOID`}
              </pre>
            </div>
          </section>

          {/* Error Codes */}
          <section className="mb-12">
            <h2 className="text-xl font-semibold text-white mb-4">Error Codes</h2>
            <div className="bg-dark-900 border border-dark-700 rounded-xl overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-dark-700 text-dark-400">
                    <th className="text-left px-4 py-3">Status</th>
                    <th className="text-left px-4 py-3">Code</th>
                    <th className="text-left px-4 py-3">Description</th>
                  </tr>
                </thead>
                <tbody className="text-dark-300">
                  {[
                    { status: '400', code: 'BAD_REQUEST', desc: 'Invalid input or missing fields' },
                    { status: '401', code: 'UNAUTHORIZED', desc: 'Missing or invalid authentication' },
                    { status: '403', code: 'FORBIDDEN', desc: 'Insufficient permissions' },
                    { status: '404', code: 'NOT_FOUND', desc: 'Resource not found' },
                    { status: '429', code: 'RATE_LIMITED', desc: 'Rate limit exceeded' },
                  ].map((e) => (
                    <tr key={e.code} className="border-b border-dark-800">
                      <td className="px-4 py-2 font-mono text-yellow-400">{e.status}</td>
                      <td className="px-4 py-2 font-mono">{e.code}</td>
                      <td className="px-4 py-2">{e.desc}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </main>

        {/* Footer */}
        <footer className="border-t border-dark-700 bg-dark-900/80 py-8 mt-12">
          <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex flex-col items-center gap-4 text-xs">
              <div className="flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-dark-400">
                <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">GitHub (MIT License)</a>
                <span className="text-dark-700">·</span>
                <a href="https://www.metavert.io/privacy-policy" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Privacy Policy</a>
                <span className="text-dark-700">·</span>
                <a href="https://www.metavert.io/terms-of-service" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Terms of Service</a>
                <span className="text-dark-700">·</span>
                <a href="/research" className="hover:text-white transition-colors">Research Citations</a>
                <span className="text-dark-700">·</span>
                <a href="/api/v1/docs" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">API Reference (MD)</a>
              </div>
              <p className="text-dark-500">&copy; 2026 Metavert LLC. All content licensed under <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer" className="text-dark-400 hover:text-white transition-colors">Creative Commons Attribution 4.0</a>.</p>
            </div>
          </div>
        </footer>
      </div>
    )
  }

  if (showResearch) {
    return (
      <div className="min-h-screen flex flex-col">
        {/* Header */}
        <header className="sticky top-0 z-50 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800">
          <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <a href="/" className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                  <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
                  </svg>
                </div>
                <h1 className="text-xl font-semibold tracking-tight text-white">LLM Optimizer</h1>
              </a>
            </div>
            <button onClick={() => { if (isResearchURL) { window.location.href = '/' } else { setShowResearch(false) } }} className="text-dark-400 hover:text-white text-sm transition-colors cursor-pointer flex items-center gap-1.5">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" /></svg>
              Back to App
            </button>
          </div>
        </header>

        {/* Research Citations Content */}
        <main className="flex-1 max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
          <h2 className="text-3xl font-bold text-white mb-2">Research Citations</h2>
          <p className="text-dark-400 text-sm mb-8">The research underpinning LLM Optimizer's analysis methodology. All scoring frameworks, dimension weights, and recommendations are derived from peer-reviewed academic work and validated practitioner research.</p>

          {/* Research Digest */}
          <section className="mb-12 bg-dark-900/60 border border-dark-800 rounded-2xl p-6 sm:p-8">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" /></svg>
              Research Digest
            </h3>
            <div className="space-y-4 text-dark-300 text-sm leading-relaxed">
              <p><strong className="text-white">Brand Recognition vs. Discovery.</strong> A key framework throughout LLM Optimizer is the distinction between <em>brand recognition</em> — how well AI represents your brand when people search for it by name — and <em>inbound discovery</em> — how often AI surfaces your brand when people search your category without prior knowledge of you. Both matter, but they require different strategies. Brand recognition improves through authority signals, earned media, and training data presence. Discovery requires appearing in category-level content, answering the questions your audience asks before they know you exist, and being present in the YouTube videos, Reddit threads, and web pages that LLMs cite for category queries.</p>
              <p>The emerging science of LLM visibility reveals a fundamental shift in how information gains authority online. The most significant recent finding comes from <a href="#nanoknow" className="text-primary-400 hover:text-primary-300 transition-colors">NanoKnow (2026)</a>, which demonstrates that content appearing frequently in training data more than doubles a model's accuracy on related questions — and that the advantage compounds when content is both memorized during training and retrievable at inference time. This means the traditional SEO playbook of optimizing for a single ranking algorithm is being replaced by a dual imperative: getting into training corpora through widespread, high-quality publication, while simultaneously remaining citable through structured, authoritative web presence.</p>
              <p>Across the research, a consistent pattern emerges: AI search engines overwhelmingly favor earned media over brand-owned content, citing third-party sources <a href="#geo-toronto" className="text-primary-400 hover:text-primary-300 transition-colors">72-92% of the time</a>. Content that includes quotations from authoritative sources gains <a href="#geo-princeton" className="text-primary-400 hover:text-primary-300 transition-colors">+41% visibility</a> — the single most effective optimization technique identified. Meanwhile, YouTube has rapidly become the dominant social citation source for LLMs, with its <a href="#youtube-citations" className="text-primary-400 hover:text-primary-300 transition-colors">share doubling to 39%</a> between August and December 2024. Critically, <a href="#livecc" className="text-primary-400 hover:text-primary-300 transition-colors">video LLMs process content through transcripts</a>, not visual analysis — a 7B model trained on YouTube transcripts outperformed 72B models, proving that transcript quality matters far more than production value.</p>
              <p>Reddit has emerged as the <a href="#youtube-citations" className="text-primary-400 hover:text-primary-300 transition-colors">#2 social citation source for LLMs</a>, with unique authority dynamics. Reddit was foundational in LLM training through datasets like <a href="#reddit-webtext" className="text-primary-400 hover:text-primary-300 transition-colors">WebText</a> and the <a href="#reddit-common-crawl" className="text-primary-400 hover:text-primary-300 transition-colors">Common Crawl</a>, and continues through <a href="#reddit-data-deals" className="text-primary-400 hover:text-primary-300 transition-colors">$60M (Google) and $70M (OpenAI) annual licensing deals</a>. Unlike YouTube's channel-centric authority, Reddit's influence comes from <a href="#reddit-community-consensus" className="text-primary-400 hover:text-primary-300 transition-colors">multi-user validation</a> — upvoted comment consensus, especially in "best X for Y" recommendation threads, creates credibility signals that LLMs weight heavily. The <a href="#geo-toronto" className="text-primary-400 hover:text-primary-300 transition-colors">Toronto GEO paper</a> classifies Reddit as "Social" — a category AI search engines suppress in direct citations — yet Reddit's pervasive presence in training data means it heavily shapes baseline model knowledge even when not explicitly cited.</p>
              <p>A critical "two-world" split has emerged between Google AI Overviews and standalone LLMs. <a href="#ahrefs-aio-citations" className="text-primary-400 hover:text-primary-300 transition-colors">76% of AI Overview citations</a> pull from top-10 organic pages — making traditional search rankings the primary signal for AIO inclusion. But for standalone LLMs like ChatGPT, <a href="#ahrefs-ai-search-overlap" className="text-primary-400 hover:text-primary-300 transition-colors">only 12% of cited URLs rank in Google's top 10</a>. The <a href="#ahrefs-75k-brands" className="text-primary-400 hover:text-primary-300 transition-colors">strongest predictor of AI citation across platforms is YouTube mentions (0.737 correlation)</a>, followed by web mentions (0.664) — not backlinks. Meanwhile, content freshness has become a significant signal: AI assistants cite content that is <a href="#ahrefs-freshness" className="text-primary-400 hover:text-primary-300 transition-colors">25.7% newer</a> than traditional search results, and <a href="#seer-recency" className="text-primary-400 hover:text-primary-300 transition-colors">65% of AI bot crawl hits target content less than a year old</a>. The <a href="#cloudflare-ai-crawlers" className="text-primary-400 hover:text-primary-300 transition-colors">explosive growth of AI crawlers (GPTBot up 305% YoY)</a> makes robots.txt policy a direct lever for AI visibility.</p>
              <p>However, this new landscape comes with important caveats. Citation accuracy across AI answer engines remains <a href="#false-promise" className="text-primary-400 hover:text-primary-300 transition-colors">surprisingly poor (49-68%)</a>, with nearly a third of claims lacking any source backing. Citation concentration follows power-law dynamics, where the <a href="#news-citing-patterns" className="text-primary-400 hover:text-primary-300 transition-colors">top 20 sources capture 28-67% of all citations</a>. And LLMs exhibit strong <a href="#lost-in-the-middle" className="text-primary-400 hover:text-primary-300 transition-colors">positional bias</a>, reliably attending to content at the beginning and end of context while ignoring the middle. Together, these findings inform LLM Optimizer's scoring frameworks across answer optimization, video authority, Reddit authority, and search visibility analysis.</p>
            </div>
          </section>

          {/* Source Papers */}
          <section className="mb-12">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" /></svg>
              Source Papers
            </h3>
            <div className="space-y-3">
              {[
                { id: 'lost-in-the-middle', title: 'Lost in the Middle: How Language Models Use Long Contexts', venue: 'TACL 2024', url: 'https://arxiv.org/abs/2307.03172', contribution: 'Position bias in LLM context windows — U-shaped attention curve where content at the beginning and end is reliably used while middle content is ignored' },
                { id: 'geo-princeton', title: 'GEO: Generative Engine Optimization', venue: 'Princeton / KDD 2024', url: 'https://arxiv.org/abs/2311.09735', contribution: 'Tested 9 content optimization strategies on 10,000 queries. Quotations (+41%), statistics (+33%), and fluency (+29%) are the most effective methods for improving LLM citation visibility' },
                { id: 'nanoknow', title: 'NanoKnow: Probing LLM Knowledge by Linking Training Data to Answers', venue: '2026', url: 'https://arxiv.org/abs/2602.20122', contribution: 'Training data frequency more than doubles model accuracy. Even with oracle RAG, models score ~11 points higher on questions with answers in training data' },
                { id: 'geo-toronto', title: 'GEO: How to Dominate AI Search — Source Preferences', venue: 'U of Toronto 2025', url: 'https://arxiv.org/abs/2509.08919', contribution: 'AI search engines cite earned media 72-92% of the time vs. 18-27% for brand-owned content. AI citations overlap with Google results only 15-50%' },
                { id: 'youtube-citations', title: 'YouTube vs Reddit AI Citations', venue: 'Adweek / Bluefish / Emberos / Goodie AI, 2025', url: 'https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations', contribution: 'YouTube appears in 16% of LLM answers (vs. 10% for Reddit). YouTube\'s social citation share doubled from 18.9% to 39.2% between Aug-Dec 2024' },
                { id: 'news-citing-patterns', title: 'News Source Citing Patterns in AI Search Systems', venue: '2025', url: 'https://arxiv.org/abs/2507.05301', contribution: 'Citation concentration and gatekeeping dynamics across 366K citations. Top 20 sources capture 28-67% of all citations (Gini 0.69-0.83)' },
                { id: 'livecc', title: 'LiveCC: Learning Video LLM with Streaming Speech Transcription', venue: 'CVPR 2025', url: 'https://arxiv.org/abs/2504.16030', contribution: 'How video LLMs are trained from ASR transcripts. A 7B model trained on YouTube transcripts surpassed 72B models, proving transcript quality matters more than model size' },
                { id: 'false-promise', title: 'The False Promise of Factual and Verifiable Source-Cited Responses', venue: '2024', url: 'https://arxiv.org/abs/2410.22349', contribution: 'Citation accuracy ranges 49-68% across answer engines. 23-32% of claims have no source backing. Perplexity generates one-sided answers 83.4% of the time' },
                { id: 'reddit-webtext', title: 'Language Models are Unsupervised Multitask Learners', venue: 'OpenAI, 2019 (Radford et al.)', url: 'https://cdn.openai.com/better-language-models/language_models_are_unsupervised_multitask_learners.pdf', contribution: 'Introduced WebText, a dataset of 8 million Reddit posts with 3+ karma score, as the foundational training corpus for GPT-2. Demonstrated that Reddit\'s community curation mechanism (karma voting) effectively serves as a quality filter for large-scale language model training data.' },
                { id: 'reddit-common-crawl', title: 'Consent in Crisis: The Rapid Decline of the AI Data Commons', venue: 'ACM FAccT 2024 (Longpre et al.)', url: 'https://dl.acm.org/doi/10.1145/3630106.3659033', contribution: 'Comprehensive audit of AI training data sources documenting Reddit\'s persistent prominence in Common Crawl and other web corpora. Found that robots.txt restrictions increased 25%+ from 2023-2024 as sites restricted AI crawling, while Reddit data remained broadly available through licensing agreements.' },
                { id: 'reddit-data-deals', title: 'Reddit Data Licensing: Google and OpenAI Deals', venue: 'Reuters / The Verge, 2024', url: 'https://www.reuters.com/technology/reddit-ai-content-licensing-deal-google-2024-02-22/', contribution: 'Google pays $60M/year and OpenAI $70M/year for Reddit data access. Reddit\'s API was locked down in 2023. Active litigation: Reddit v. Anthropic, Reddit v. Perplexity (scraping claims).' },
                { id: 'reddit-community-consensus', title: 'Community Consensus as LLM Authority Signal', venue: 'Bluefish Labs / Emberos Research, 2025', url: 'https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations', contribution: 'Reddit\'s multi-user validation (upvotes, comment consensus) creates credibility signals single-author content cannot match. "Best X for Y" recommendation threads are among the most influential for LLM comparison queries.' },
                { id: 'ahrefs-aio-citations', title: 'AI Overview Citations and Search Rankings', venue: 'Ahrefs, 2025', url: 'https://ahrefs.com/blog/search-rankings-ai-citations/', contribution: '76% of AI Overview citations pull from top-10 organic pages. Median organic ranking for a cited URL is position 3. 86% of citations come from within the top 100 organic results.' },
                { id: 'ahrefs-ai-search-overlap', title: 'AI Search Overlap: How AI Citations Differ from Google', venue: 'Ahrefs, 2025', url: 'https://ahrefs.com/blog/ai-search-overlap/', contribution: 'Only 12% of standalone LLM citations overlap with Google\'s top 10. Perplexity shows 28.6% overlap. 80%+ of ChatGPT/Claude/Gemini citations come from pages not ranking in Google at all.' },
                { id: 'ahrefs-75k-brands', title: 'AI Brand Visibility Correlations (75K Brands)', venue: 'Ahrefs, 2025', url: 'https://ahrefs.com/blog/ai-brand-visibility-correlations/', contribution: 'YouTube mentions (0.737) and web mentions (0.664) are the strongest correlators with AI visibility. Brand search volume (0.334) outperforms backlinks (0.37). Top 25% brands get 12x more AIO mentions.' },
                { id: 'ahrefs-freshness', title: 'Do AI Assistants Prefer to Cite Fresh Content?', venue: 'Ahrefs, 2025 (17M citations)', url: 'https://ahrefs.com/blog/do-ai-assistants-prefer-to-cite-fresh-content/', contribution: 'AI assistants cite content 25.7% newer than traditional search. ChatGPT: avg 1,023 days old. Perplexity pulls ~50% from current year. Google AIOs counter-trend: prefer older authoritative content.' },
                { id: 'seer-recency', title: 'AI Brand Visibility and Content Recency', venue: 'Seer Interactive, 2025', url: 'https://www.seerinteractive.com/insights/study-ai-brand-visibility-and-content-recency', contribution: '65% of AI bot crawl hits target content published within the past year. 85% of AIO citations from last 2 years. 94% from last 5 years.' },
                { id: 'llm-recency-bias', title: 'Do Large Language Models Favor Recent Content?', venue: 'arXiv, September 2025', url: 'https://arxiv.org/html/2509.11353v1', contribution: 'LLMs consistently promote "fresh" passages. Top-10 mean publication year shifts forward by up to 4.78 years. Individual items move up to 95 ranking positions based on recency signals alone.' },
                { id: 'cloudflare-ai-crawlers', title: 'From Googlebot to GPTBot: Who\'s Crawling Your Site', venue: 'Cloudflare, 2025', url: 'https://blog.cloudflare.com/from-googlebot-to-gptbot-whos-crawling-your-site-in-2025/', contribution: 'GPTBot grew 305% YoY. OpenAI crawl-to-referral ratio: 1,700:1. Anthropic: 73,000:1. ~21% of top-1000 sites block GPTBot. Training crawls = 80% of AI bot activity.' },
                { id: 'semrush-aio', title: 'AI Overviews Study: 200,000 Keywords', venue: 'Semrush, 2025', url: 'https://www.semrush.com/blog/semrush-ai-overviews-study/', contribution: 'Reddit (40.1%) and Wikipedia (26.3%) dominate AIO citations. 80% of AIO responses target informational queries. 82% appear for keywords with <1,000 monthly searches.' },
              ].map((paper, i) => (
                <a key={i} id={paper.id} href={paper.url} target="_blank" rel="noopener noreferrer" className="block bg-dark-900/50 border border-dark-800 rounded-xl p-4 hover:border-primary-500/40 transition-colors group scroll-mt-24">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex-1 min-w-0">
                      <p className="text-white text-sm font-medium group-hover:text-primary-300 transition-colors">{paper.title}</p>
                      <p className="text-dark-500 text-xs mt-0.5">{paper.venue}</p>
                      <p className="text-dark-400 text-xs mt-2 leading-relaxed">{paper.contribution}</p>
                    </div>
                    <svg className="w-4 h-4 text-dark-600 group-hover:text-primary-400 shrink-0 mt-1 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" /></svg>
                  </div>
                </a>
              ))}
            </div>
          </section>

          {/* Answer Optimization Framework */}
          <section className="mb-12">
            <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 3v11.25A2.25 2.25 0 006 16.5h2.25M3.75 3h-1.5m1.5 0h16.5m0 0h1.5m-1.5 0v11.25A2.25 2.25 0 0118 16.5h-2.25m-7.5 0h7.5m-7.5 0l-1 3m8.5-3l1 3m0 0l.5 1.5m-.5-1.5h-9.5m0 0l-.5 1.5" /></svg>
              Answer Optimization Scoring Framework
            </h3>
            <p className="text-dark-400 text-sm mb-6">Each optimization report scores how likely an LLM is to surface and cite a website's answer across four research-backed dimensions.</p>

            <div className="space-y-4">
              {[
                {
                  name: 'Content Authority', weight: '30%', color: 'primary',
                  source: 'GEO (Princeton/KDD 2024)',
                  desc: 'Measures the presence of quotations from authoritative sources (+41% visibility), statistical evidence (+33%), source citations (+28%), fluency (+29%), and technical terminology (+19%). Penalizes keyword stuffing (-9%).',
                },
                {
                  name: 'Structural Optimization', weight: '20%', color: 'accent-purple',
                  source: 'Lost in the Middle (TACL 2024) + GEO (Toronto 2025)',
                  desc: 'Evaluates answer prominence (front-loaded vs. buried), content conciseness, machine-readable structure (Schema.org, tables, comparison formats), and justification language that explains "why" rather than just "what."',
                },
                {
                  name: 'Source Authority', weight: '30%', color: 'accent-emerald',
                  source: 'GEO (Toronto 2025)',
                  desc: 'Assesses third-party coverage and earned media presence. AI search engines cite earned media 72-92% of the time. Evaluates cross-engine consistency since different AI providers cite substantially different sources (similarity only 0.11-0.58).',
                },
                {
                  name: 'Knowledge Persistence', weight: '20%', color: 'amber',
                  source: 'NanoKnow (2026)',
                  desc: 'Measures how deeply information is embedded in model training data. Answer frequency more than doubles accuracy. Content that is both in training data AND retrievable at inference compounds advantage by ~11 percentage points. Clear, educational writing outperforms natural text by 19+ points.',
                },
              ].map((dim, i) => (
                <div key={i} className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-white font-medium text-sm">{dim.name}</h4>
                    <span className="text-xs px-2 py-0.5 rounded-full bg-dark-800 text-dark-400 border border-dark-700">{dim.weight}</span>
                  </div>
                  <p className="text-dark-500 text-[11px] mb-2">Source: {dim.source}</p>
                  <p className="text-dark-400 text-xs leading-relaxed">{dim.desc}</p>
                </div>
              ))}
            </div>
          </section>

          {/* Video Authority Framework */}
          <section className="mb-12">
            <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>
              Video Authority Scoring Framework
            </h3>
            <p className="text-dark-400 text-sm mb-6">Video analysis evaluates YouTube presence across four pillars, grounded in the finding that LLMs process video through transcripts, not visual content.</p>

            <div className="space-y-4">
              {[
                {
                  name: 'Transcript Authority', weight: '30%',
                  source: 'LiveCC (CVPR 2025) + GEO (Princeton 2024)',
                  desc: 'Transcript quality is the dominant signal for LLM visibility. Evaluates keyword alignment, quotability (standalone citable statements get +41% visibility per GEO), information density, and caption availability. Videos without captions are effectively invisible to LLMs.',
                },
                {
                  name: 'Topical Dominance', weight: '25%',
                  source: 'AI Search Arena (2025) + GEO (Toronto 2025)',
                  desc: 'Measures topic coverage breadth and depth, share of voice across video content in the space, content gaps representing first-mover opportunities, and coverage depth (surface vs. in-depth treatment). Winner-take-all dynamics mean being first in a topic gap has outsized value.',
                },
                {
                  name: 'Citation Network', weight: '25%',
                  source: 'AI Search Arena (2025) + YouTube Citation Analysis (Adweek 2025)',
                  desc: 'Analyzes who mentions the brand, their authority level, and concentration risk. Top 20 sources capture 28-67% of all AI citations. A mention by a high-authority channel outweighs dozens of small-channel mentions. Human engagement metrics (views, subscribers) do not predict AI citation.',
                },
                {
                  name: 'Brand Narrative Quality', weight: '20%',
                  source: 'False Promise of Source-Cited Responses (2024) + Lost in the Middle (2024)',
                  desc: 'Evaluates sentiment, mention context and position (early mentions get priority per U-shaped attention), extractability (clear mentions are less likely to be misrepresented given 49-68% citation accuracy), and narrative coherence. Includes a confidence discount reflecting known citation inaccuracy rates.',
                },
              ].map((pillar, i) => (
                <div key={i} className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-white font-medium text-sm">{pillar.name}</h4>
                    <span className="text-xs px-2 py-0.5 rounded-full bg-dark-800 text-dark-400 border border-dark-700">{pillar.weight}</span>
                  </div>
                  <p className="text-dark-500 text-[11px] mb-2">Source: {pillar.source}</p>
                  <p className="text-dark-400 text-xs leading-relaxed">{pillar.desc}</p>
                </div>
              ))}
            </div>
          </section>

          {/* Reddit Authority Framework */}
          <section className="mb-12">
            <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
              <svg className="w-5 h-5 text-orange-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" /></svg>
              Reddit Authority Scoring Framework
            </h3>
            <p className="text-dark-400 text-sm mb-6">Reddit analysis evaluates community discussion across four pillars, grounded in Reddit's unique role as a multi-user validation platform for LLM training data.</p>

            <div className="space-y-4">
              {[
                {
                  name: 'Presence', weight: '25%',
                  source: 'Reddit Training Data Analysis (2024-2025) + GEO (Toronto 2025)',
                  desc: 'Volume and breadth of brand mentions across relevant subreddits. Measures total mentions, unique subreddits reached, and mention trend over time. High presence in topic-specific subreddits carries more weight than general discussion.',
                },
                {
                  name: 'Sentiment & Recommendations', weight: '25%',
                  source: 'Community Consensus Research (Bluefish/Emberos 2025)',
                  desc: 'Community tone and recommendation strength. Evaluates positive/negative sentiment balance, recommendation rate in "best X for Y" threads, and the specific praise/criticism themes that shape LLM perception.',
                },
                {
                  name: 'Competitive Positioning', weight: '25%',
                  source: 'GEO (Toronto 2025) + Reddit Community Analysis',
                  desc: 'Head-to-head positioning against competitors in comparison threads. Measures win rate, cited differentiators, and competitor advantages not countered — these directly shape LLM comparison responses.',
                },
                {
                  name: 'Training Signal Strength', weight: '25%',
                  source: 'NanoKnow (2026) + Reddit Data Licensing (2024)',
                  desc: 'Likelihood that Reddit discussions will influence LLM training. High-upvote threads in authoritative subreddits with deep comment engagement create the strongest training signals. Reddit data is actively licensed to OpenAI and Google.',
                },
              ].map((pillar, i) => (
                <div key={i} className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-white font-medium text-sm">{pillar.name}</h4>
                    <span className="text-xs px-2 py-0.5 rounded-full bg-dark-800 text-dark-400 border border-dark-700">{pillar.weight}</span>
                  </div>
                  <p className="text-dark-500 text-[11px] mb-2">Source: {pillar.source}</p>
                  <p className="text-dark-400 text-xs leading-relaxed">{pillar.desc}</p>
                </div>
              ))}
            </div>
          </section>

          {/* Search Visibility Framework */}
          <section className="mb-12">
            <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
              Search Visibility Scoring Framework
            </h3>
            <p className="text-dark-400 text-sm mb-6">Search visibility analysis evaluates how search-related signals affect whether AI systems will discover, index, and cite your content — bridging traditional SEO signals with AI citation dynamics. When Brand Intelligence provides category data, a fifth pillar (Category Discovery) measures whether people searching your category — without knowing your brand — can find you.</p>

            <div className="space-y-4">
              {[
                {
                  name: 'AI Overview Readiness', weight: '30%',
                  source: 'Ahrefs AIO Citations Study (2025) + Semrush AIO Study (2025)',
                  desc: '76% of AI Overview citations pull from top-10 organic pages. Evaluates organic ranking presence, structured data (Schema.org, JSON-LD), content format alignment with AIO-preferred informational queries, and answer prominence (front-loaded concise answers). AIOs favor long-tail keywords — 82% appear for terms with <1,000 monthly searches.',
                },
                {
                  name: 'Crawl Accessibility', weight: '20%',
                  source: 'Cloudflare AI Crawler Report (2025) + Consent in Crisis (ACM FAccT 2024)',
                  desc: 'GPTBot grew 305% YoY with a crawl-to-referral ratio of 1,700:1. Evaluates robots.txt policy for AI crawlers (GPTBot, ClaudeBot, PerplexityBot and their SearchBot variants), sitemap completeness, and render accessibility. Blocking training bots while allowing search bots is a valid strategy; blocking everything eliminates AI visibility.',
                },
                {
                  name: 'Brand Search Momentum', weight: '25%',
                  source: 'Ahrefs 75K-Brand Study (2025) + Google Trends API (2025)',
                  desc: 'Brand search volume has a 0.334 correlation with AI citation frequency — but web mentions (0.664) and YouTube mentions (0.737) are stronger. Winner-takes-all: top 25% brands average 169 AIO mentions vs. 14 for the 50th-75th percentile. Evaluates brand search trends, entity recognition, and competitive positioning.',
                },
                {
                  name: 'Content Freshness', weight: '25%',
                  source: 'Ahrefs 17M Citations Study (2025) + Seer Interactive (2025) + arXiv Recency Bias (2025)',
                  desc: 'AI assistants cite content 25.7% newer than traditional search. 65% of AI bot hits target content <1 year old. Freshness signals can move items up to 95 ranking positions in LLM reranking. Evaluates content age, update frequency, freshness signals (dates, last-modified), and content decay risk. Note: Google AIOs counter-trend, preferring older authoritative content.',
                },
                {
                  name: 'Category Discovery', weight: '20% (when categories available)',
                  source: 'Brand Intelligence categories + target queries',
                  desc: 'When Brand Intelligence provides category keywords and intent queries, a fifth pillar evaluates how visible the brand is in category-level searches — queries where users search the category without knowing the brand. This measures discovery potential: does the brand appear when someone searches "best [category] tools" or "[use case] solutions"? Sub-metrics include category visibility, intent coverage, competitor gap, and discovery potential. Weights rebalance to 25/15/20/20/20 across all five pillars.',
                },
              ].map((pillar, i) => (
                <div key={i} className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-white font-medium text-sm">{pillar.name}</h4>
                    <span className="text-xs px-2 py-0.5 rounded-full bg-dark-800 text-dark-400 border border-dark-700">{pillar.weight}</span>
                  </div>
                  <p className="text-dark-500 text-[11px] mb-2">Source: {pillar.source}</p>
                  <p className="text-dark-400 text-xs leading-relaxed">{pillar.desc}</p>
                </div>
              ))}
            </div>
          </section>

          {/* Key Research Findings */}
          <section className="mb-12">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 001.5-.189m-1.5.189a6.01 6.01 0 01-1.5-.189m3.75 7.478a12.06 12.06 0 01-4.5 0m3.75 2.383a14.406 14.406 0 01-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 10-7.517 0c.85.493 1.509 1.333 1.509 2.316V18" /></svg>
              Key Research Findings
            </h3>
            <div className="space-y-3">
              {[
                { finding: 'Quotations are the single most effective optimization method', detail: 'Adding quotes from authoritative sources improves LLM visibility by 41%, more than any other technique tested on 10,000 queries. Statistics (+33%) and fluency (+29%) follow.', source: 'GEO, Princeton/KDD 2024', anchor: '#geo-princeton' },
                { finding: 'Lower-ranked sites benefit disproportionately', detail: 'Rank-5 sites saw +115% visibility improvement from citing sources, while rank-1 sites saw -30%. Generative engines can be more democratic than traditional search for well-optimized content.', source: 'GEO, Princeton/KDD 2024', anchor: '#geo-princeton' },
                { finding: 'AI search overwhelmingly favors earned media', detail: 'AI search engines cite independent third-party sources 72-92% of the time, compared to only 18-27% for brand-owned content and virtually 0% for social content.', source: 'GEO, Toronto 2025', anchor: '#geo-toronto' },
                { finding: 'Training data frequency more than doubles accuracy', detail: 'Models are more than twice as accurate on questions whose answers appear frequently (51+ documents) in training data vs. rarely (1-5 documents). Being in training data AND retrievable compounds advantage.', source: 'NanoKnow, 2026', anchor: '#nanoknow' },
                { finding: 'YouTube is the #1 social citation source for LLMs', detail: 'YouTube\'s share of social citations doubled from 18.9% to 39.2% in just 5 months. It generates 18x more AI citations than Instagram and 50x more than TikTok. Views and subscriber counts do not predict AI citation.', source: 'Adweek / Bluefish / Emberos / Goodie AI, 2025', anchor: '#youtube-citations' },
                { finding: 'Video LLMs are trained on transcripts, not visual content', detail: 'A 7B model trained on YouTube transcripts outperformed 72B models. No captions = invisible to LLMs. Transcript quality is the dominant factor, not production value.', source: 'LiveCC, CVPR 2025', anchor: '#livecc' },
                { finding: 'Content position follows a U-shaped attention curve', detail: 'LLMs reliably use content at the beginning and end of their context window but effectively ignore the middle. Front-loading key information is critical for citation.', source: 'Lost in the Middle, TACL 2024', anchor: '#lost-in-the-middle' },
                { finding: 'AI citation accuracy is surprisingly poor', detail: 'Perplexity achieves only 49% citation accuracy; You.com 68%; BingChat 66%. 23-32% of relevant statements have no source backing. Systems display more sources than they actually use.', source: 'False Promise of Source-Cited Responses, 2024', anchor: '#false-promise' },
                { finding: 'Reddit is the #2 social citation source and foundational training data', detail: 'Reddit accounts for 10-40% of AI social citations depending on platform/timeframe. WebText (GPT-2 training) was built from 8M Reddit posts with 3+ karma. Reddit remains pervasive in Common Crawl and is actively licensed to Google ($60M/yr) and OpenAI ($70M/yr).', source: 'Multiple sources, 2024-2025', anchor: '#reddit-training-data' },
                { finding: 'Community consensus creates unique credibility signals', detail: 'Upvoted comment threads, especially "best X for Y" recommendation discussions, create multi-user validation that LLMs weight heavily. This multi-user signal cannot be replicated by single-author content.', source: 'Bluefish Labs / Emberos, 2025', anchor: '#reddit-community-consensus' },
                { finding: 'AI Overviews strongly favor top-ranked pages', detail: '76% of Google AI Overview citations come from top-10 organic pages, with median cited position at rank 3. But standalone LLMs (ChatGPT, Claude, Gemini) show only 12% overlap — they cite fundamentally different sources.', source: 'Ahrefs, 2025', anchor: '#ahrefs-aio-citations' },
                { finding: 'Web mentions outperform backlinks for AI visibility', detail: 'Brand web mentions (0.664 correlation) and YouTube mentions (0.737) are far stronger predictors of AI citation than backlinks (0.37). Top 25% brands by web mentions get 12x more AI Overview mentions than the 50-75th percentile.', source: 'Ahrefs 75K Brands Study, 2025', anchor: '#ahrefs-75k-brands' },
                { finding: 'AI assistants strongly prefer fresh content', detail: 'Content cited by AI assistants is 25.7% newer on average than traditional search results. 65% of AI bot crawl hits target content less than 1 year old. Freshness signals can shift LLM ranking positions by up to 95 places.', source: 'Ahrefs (17M citations) + arXiv, 2025', anchor: '#ahrefs-freshness' },
                { finding: 'AI crawlers are growing explosively', detail: 'GPTBot grew 305% YoY, with OpenAI\'s crawl-to-referral ratio at 1,700:1. Each major AI company now runs 3 separate bots (training, indexing, user-fetch). Blocking training bots while allowing search bots is a valid strategy.', source: 'Cloudflare, 2025', anchor: '#cloudflare-ai-crawlers' },
              ].map((item, i) => (
                <a key={i} href={item.anchor} className="block bg-dark-900/50 border border-dark-800 rounded-xl p-4 hover:border-primary-500/40 transition-colors group">
                  <p className="text-white text-sm font-medium group-hover:text-primary-300 transition-colors">{item.finding}</p>
                  <p className="text-dark-400 text-xs mt-1.5 leading-relaxed">{item.detail}</p>
                  <p className="text-dark-500 text-[11px] mt-2 flex items-center gap-1.5">{item.source} <svg className="w-3 h-3 text-dark-600 group-hover:text-primary-400 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12h15m0 0l-6.75-6.75M19.5 12l-6.75 6.75" /></svg></p>
                </a>
              ))}
            </div>
          </section>
        </main>

        {/* CTA */}
        <section className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
          <div className="bg-gradient-to-r from-primary-600/20 to-accent-purple/20 border border-primary-500/30 rounded-2xl p-8 text-center">
            <h3 className="text-xl font-bold text-white mb-2">Put this research to work</h3>
            <p className="text-dark-300 text-sm mb-6 max-w-lg mx-auto">LLM Optimizer applies these research findings automatically to analyze and optimize your brand's visibility across AI search engines.</p>
            <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
              <a href="/signup" className="inline-flex items-center gap-2 px-6 py-3 bg-primary-600 text-white rounded-xl hover:bg-primary-500 transition-colors font-semibold">
                Get Started
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" /></svg>
              </a>
              <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 px-6 py-3 border border-dark-600 text-dark-300 rounded-xl hover:border-dark-500 hover:text-white transition-colors font-semibold">
                <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
                Self-Host Free
              </a>
            </div>
            <p className="text-dark-500 text-xs mt-4">LLM Optimizer is open-source (MIT). Our hosted version supports ongoing development.</p>
          </div>
        </section>

        {/* Footer */}
        <footer className="border-t border-dark-700 bg-dark-900/80 py-8 mt-12">
          <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex flex-col items-center gap-4 text-xs">
              <div className="flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-dark-400">
                <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">GitHub (MIT License)</a>
                <span className="text-dark-700">·</span>
                <a href="https://www.metavert.io/privacy-policy" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Privacy Policy</a>
                <span className="text-dark-700">·</span>
                <a href="https://www.metavert.io/terms-of-service" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Terms of Service</a>
                <span className="text-dark-700">·</span>
                <a href="/docs" className="hover:text-white transition-colors">API Docs</a>
                <span className="text-dark-700">·</span>
                <a href="/api/v1/docs" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">API Reference (MD)</a>
              </div>
              <p className="text-dark-500">&copy; 2026 Metavert LLC. All content licensed under <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer" className="text-dark-400 hover:text-white transition-colors">Creative Commons Attribution 4.0</a>.</p>
            </div>
          </div>
        </footer>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="sticky top-0 z-50 bg-dark-900/80 backdrop-blur-xl border-b border-dark-800">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <a href="/" className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary-500 to-accent-purple flex items-center justify-center">
                <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
                </svg>
              </div>
              <h1 className="text-xl font-semibold tracking-tight text-white">
                LLM Optimizer
              </h1>
            </a>
            {readOnly && selectedDomain && (
              <span className="text-dark-400 text-sm ml-2">· {selectedDomain}</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {!readOnly && saasEnabled && user && (
              <>
                {user.isRootTenant && (user.role === 'owner' || user.role === 'admin') && (
                  <a href="/last" className="px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">Admin</a>
                )}
                <a href="/last/team" className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
                  </svg>
                  Team
                </a>
                <a href="/last/plan" className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" />
                  </svg>
                  Plan
                </a>
                {showCredits && (
                  <a href="/last/plan" className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-dark-800 border border-dark-700 text-sm text-dark-300 hover:text-white hover:border-primary-500/30 transition-colors" title="Usage credits">
                    <svg className="w-4 h-4 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
                    </svg>
                    <span className="font-medium">{tenantCredits.toLocaleString()}</span>
                  </a>
                )}
                <a href="/last/messages" className="relative px-2 py-1.5 text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75v-.7V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0" />
                  </svg>
                  {unreadCount > 0 && (
                    <span className="absolute -top-0.5 -right-0.5 w-4 h-4 bg-primary-500 text-white text-[10px] font-bold rounded-full flex items-center justify-center">{unreadCount > 9 ? '9+' : unreadCount}</span>
                  )}
                </a>
                <a href="/last/settings" className="px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors truncate max-w-[120px]">{user.displayName || user.email}</a>
                <button
                  onClick={() => clearAuth()}
                  className="text-dark-400 hover:text-white transition-colors"
                  title="Sign out"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
                  </svg>
                </button>
              </>
            )}
            {saasEnabled && !user && (
              <>
                <a href="/login" className="px-3 py-1.5 text-sm text-dark-400 hover:text-white hover:bg-dark-800/50 rounded-lg transition-colors">Sign In</a>
                <a href="/signup" className="px-3 py-1.5 text-sm text-white bg-primary-500 hover:bg-primary-600 rounded-lg transition-colors">Sign Up</a>
              </>
            )}
            {(!saasEnabled || user) && (() => {
              // Ghost placeholder while config/auth/API key status is loading
              if (!configLoaded || (saasEnabled && user && apiKeyStatus === 'loading')) {
                return (
                  <div className="flex items-center gap-1.5 px-3 py-1.5">
                    <span className="w-2.5 h-2.5 rounded-full bg-dark-700 animate-pulse" />
                    <span className="w-12 h-3.5 rounded bg-dark-700 animate-pulse hidden sm:inline-block" />
                  </div>
                )
              }
              // Key status overrides for SaaS users
              if (saasEnabled && user && (apiKeyStatus === 'unconfigured')) {
                return (
                  <a
                    href="/last/settings"
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-400 hover:bg-amber-500/20 transition-colors text-sm font-medium"
                    title="Configure your API key"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                    <span className="hidden sm:inline">API Key Needed</span>
                  </a>
                )
              }
              if (saasEnabled && user && apiKeyStatus === 'invalid') {
                return (
                  <a
                    href="/last/settings"
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 hover:bg-red-500/20 transition-colors text-sm"
                    title="API key is invalid"
                  >
                    <span className="w-2.5 h-2.5 rounded-full bg-red-400" />
                    <span className="text-sm">Key Invalid</span>
                  </a>
                )
              }
              if (saasEnabled && user && apiKeyStatus === 'no_credits') {
                return (
                  <button
                    onClick={() => setActiveTab('status')}
                    className={`relative flex items-center gap-2 px-3 py-1.5 rounded-lg transition-colors cursor-pointer ${
                      activeTab === 'status' ? 'bg-dark-800 text-white' : 'hover:bg-dark-800/50 text-dark-400'
                    }`}
                    title="API key needs credits"
                  >
                    <span className="relative">
                      {healthHistory.length > 0 ? (() => {
                        const latest = healthHistory[0]
                        const anyAvailable = latest.models.some(m => m.status === 'available')
                        const allError = latest.models.every(m => m.status === 'error')
                        return <span className={`w-2.5 h-2.5 rounded-full inline-block ${anyAvailable ? 'bg-emerald-400' : allError ? 'bg-red-400' : 'bg-amber-400'}`} />
                      })() : <span className="w-2.5 h-2.5 rounded-full inline-block bg-dark-600" />}
                      <span className="absolute -top-1.5 -right-2 text-amber-400 text-[10px] font-bold">$</span>
                    </span>
                    <span className="text-sm">Status</span>
                  </button>
                )
              }
              // Default: normal status light
              return (
                <button
                  onClick={() => setActiveTab('status')}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-lg transition-colors cursor-pointer ${
                    activeTab === 'status' ? 'bg-dark-800 text-white' : 'hover:bg-dark-800/50 text-dark-400'
                  }`}
                >
                  {healthHistory.length > 0 ? (() => {
                    const latest = healthHistory[0]
                    const anyAvailable = latest.models.some(m => m.status === 'available')
                    const allError = latest.models.every(m => m.status === 'error')
                    return <span className={`w-2.5 h-2.5 rounded-full ${anyAvailable ? 'bg-emerald-400' : allError ? 'bg-red-400' : 'bg-amber-400'}`} />
                  })() : <span className="w-2.5 h-2.5 rounded-full bg-dark-600" />}
                  <span className="text-sm">Status</span>
                </button>
              )
            })()}
          </div>
        </div>
      </header>

      <main className="flex-1 max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-12 w-full">
        {/* Hero */}
        {!readOnly && (
        <div className="text-center mb-8">
          <h2 className="text-4xl sm:text-5xl font-bold text-white mb-4 leading-tight">
            Be the Brand AI Recommends
          </h2>
          <p className="text-dark-400 text-lg max-w-2xl mx-auto leading-relaxed">
            ChatGPT, Claude, Gemini, and Perplexity are replacing search for millions of people.
            We help you see how AI sees your brand — and optimize so it recommends you.
          </p>
        </div>
        )}

        {/* Tab Navigation — Domain List + tab pills */}
        <div className="flex justify-center mb-6 px-2">
          <div className="inline-flex items-center bg-dark-900/50 border border-dark-800 rounded-lg p-1 gap-1 max-w-full overflow-x-auto scrollbar-hide">
            {/* Back button — shown when viewing a report, navigates to default home view */}
            {selectedDomain && (
              <button
                onClick={() => {
                  if (sharedMode) {
                    window.location.href = '/'
                  } else {
                    setSelectedDomain('')
                    setActiveTab('analyze')
                    setUrl('')
                    setResult(null)
                    setResultMeta(null)
                    setState('idle')
                  }
                }}
                className="px-2 py-2 rounded-md text-dark-400 hover:text-white transition-all cursor-pointer"
                title="Back to home"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                </svg>
              </button>
            )}
            {!readOnly && <select
              value={selectedDomain}
              onChange={e => {
                const domain = e.target.value
                const doSwitch = () => {
                  setSelectedDomain(domain)
                  if (domain) {
                    setUrl(domain)
                    if (activeTab === 'analyze') {
                      loadBestAnalysis(domain)
                    }
                  } else {
                    // (New Domain) selected — go to Analyze tab
                    setActiveTab('analyze')
                    setUrl('')
                    setResult(null)
                    setResultMeta(null)
                    setState('idle')
                  }
                }
                if (activeTab === 'brand' && brandEditing) {
                  guardBrandNav(doSwitch)
                } else {
                  doSwitch()
                }
              }}
              className="px-3 py-2 bg-dark-800 border border-dark-700 rounded-md text-white focus:outline-none focus:border-primary-500/50 cursor-pointer text-sm max-w-[200px] truncate"
            >
              <option value="">(New Domain)</option>
              {allDomains.map(d => (
                <option key={d.domain} value={d.domain}>{d.domain.replace(/^https?:\/\//, '')}</option>
              ))}
            </select>}
            <button
              onClick={() => {
                const doSwitch = () => {
                  setActiveTab('analyze')
                  if (selectedDomain) loadBestAnalysis(selectedDomain)
                }
                activeTab === 'brand' && brandEditing ? guardBrandNav(doSwitch) : doSwitch()
              }}
              className={`px-5 py-2 rounded-md text-sm font-medium transition-all cursor-pointer shrink-0 ${
                activeTab === 'analyze'
                  ? 'bg-primary-600 text-white'
                  : 'text-dark-400 hover:text-white'
              }`}
            >
              Analyze
            </button>
            {(['todos', 'brand', 'video', 'reddit', 'search', 'optimize', 'test'] as const).map(tab => {
              const labels: Record<string, string> = { todos: 'To-Do', brand: 'Brand', video: 'YouTube', reddit: 'Reddit', search: 'Search', optimize: 'Optimize', test: 'Test' }
              const disabled = !selectedDomain
              return (
                <button
                  key={tab}
                  onClick={() => {
                    if (disabled) return
                    if (activeTab === 'brand' && brandEditing) {
                      guardBrandNav(() => setActiveTab(tab))
                    } else {
                      setActiveTab(tab)
                    }
                  }}
                  className={`px-5 py-2 rounded-md text-sm font-medium transition-all relative flex items-center gap-2 shrink-0 ${
                    disabled
                      ? 'text-dark-600 cursor-not-allowed'
                      : activeTab === tab
                        ? 'bg-primary-600 text-white cursor-pointer'
                        : 'text-dark-400 hover:text-white cursor-pointer'
                  }`}
                >
                  {labels[tab]}
                  {tab === 'todos' && selectedDomain && (() => {
                    const todoCount = todos.filter(t => t.status === 'todo' && domainKey(t.domain) === domainKey(selectedDomain)).length
                    // Include virtual brand intel todo if brand is incomplete
                    const brandPct = sharedMode
                      ? (brandEditing ? brandCompleteness : 0)
                      : (brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.completeness ?? 0)
                    const count = todoCount + (brandPct < 100 ? 1 : 0)
                    return count > 0 ? (
                      <span className="absolute -top-1.5 -right-1.5 min-w-[18px] h-[18px] flex items-center justify-center px-1 text-[10px] font-bold bg-red-500 text-white rounded-full leading-none">
                        {count > 99 ? '99+' : count}
                      </span>
                    ) : null
                  })()}
                  {tab === 'brand' && selectedDomain && (() => {
                    // In shared mode, use brandCompleteness (computed from form state) since brandList isn't populated
                    const pct = sharedMode
                      ? (brandEditing ? brandCompleteness : 0)
                      : (brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.completeness ?? 0)
                    if (pct >= 100) return null
                    if (pct > 0) return (
                      <span className="absolute -top-1.5 -right-1.5 min-w-[18px] h-[18px] flex items-center justify-center px-1 text-[10px] font-bold bg-amber-500 text-white rounded-full leading-none">
                        {pct}%
                      </span>
                    )
                    // Don't show empty dot if brand profile exists (shared mode with full profile)
                    if (sharedMode && brandProfile) return null
                    return <span className="absolute -top-1 -right-1 w-2.5 h-2.5 bg-amber-500 rounded-full" />
                  })()}
                  {tab === 'video' && videoAnalyzing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                  {tab === 'reddit' && redditAnalyzing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                  {tab === 'search' && searchAnalyzing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                  {tab === 'optimize' && optimizing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                  {tab === 'test' && testAnalyzing && <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />}
                  {/* Not-yet-run indicator dot */}
                  {!readOnly && selectedDomain && visibilityScore && (() => {
                    const tabComponentMap: Record<string, string> = {
                      optimize: 'Optimization', video: 'Video Authority',
                      reddit: 'Reddit Authority', search: 'Search Visibility', test: 'LLM Test'
                    }
                    const componentName = tabComponentMap[tab]
                    if (!componentName) return null
                    const component = visibilityScore.components.find(c => c.name === componentName)
                    if (component && !component.available) {
                      return <span className="absolute -top-1 -right-1 w-2.5 h-2.5 bg-amber-500 rounded-full" />
                    }
                    return null
                  })()}
                </button>
              )
            })}
            {selectedDomain && !readOnly && (
              optList.some(o => domainKey(o.domain) === domainKey(selectedDomain)) ||
              videoAnalysisList.some(v => domainKey(v.domain) === domainKey(selectedDomain)) ||
              redditAnalysisList.some(r => domainKey(r.domain) === domainKey(selectedDomain)) ||
              searchAnalysisList.some(s => domainKey(s.domain) === domainKey(selectedDomain))
            ) && (
              <button
                onClick={generatePDF}
                disabled={pdfGenerating}
                className="px-2 py-2 rounded-md text-dark-400 hover:text-white transition-all cursor-pointer ml-1 border-l border-dark-700 pl-2 relative"
                title={pdfGenerating ? pdfProgress || 'Generating report...' : 'Download PDF Report'}
              >
                {pdfGenerating ? (
                  <div className="w-4 h-4 border-2 border-dark-600 border-t-primary-400 rounded-full animate-spin" />
                ) : (
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                  </svg>
                )}
              </button>
            )}
            {readOnly && sharedShareId && (
              <a
                href={`/api/share/${sharedShareId}/pdf`}
                className="px-2 py-2 rounded-md text-dark-400 hover:text-white transition-all cursor-pointer ml-1 border-l border-dark-700 pl-2"
                title="Download PDF Report"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                </svg>
              </a>
            )}
          </div>
        </div>

        {/* URL entry + Go button — always on Analyze tab */}
        {activeTab === 'analyze' && <div className="max-w-xl mx-auto mb-8 flex items-center gap-2">
          <input
            type="text"
            value={url}
            onChange={e => setUrl(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && state !== 'analyzing') {
                if (saasEnabled && !user) { trackPageView(window.location.href, 'Login Modal'); setLoginModal(true) }
                else if (sharedMode) { window.location.href = '/' }
                else { analyze() }
              }
            }}
            placeholder="example.com"
            disabled={state === 'analyzing'}
            className="flex-1 px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors disabled:opacity-50 text-center"
          />
          {state === 'analyzing' ? (
            <button
              onClick={cancel}
              className="px-6 py-2.5 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all cursor-pointer shrink-0"
            >
              Cancel
            </button>
          ) : (
            <button
              onClick={() => {
                if (saasEnabled && !user) { trackPageView(window.location.href, 'Login Modal'); setLoginModal(true) }
                else if (sharedMode) { window.location.href = '/' }
                else { analyze() }
              }}
              disabled={!url.trim()}
              className="px-6 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shrink-0"
            >
              Go
            </button>
          )}
        </div>}

        {/* Quick-start hint for new users with no domains */}
        {!readOnly && activeTab === 'analyze' && !selectedDomain && allDomains.length === 0 && state === 'idle' && popularDomains.length > 0 && (
          <p className="text-center text-dark-500 text-xs -mt-4 mb-6">
            Enter your domain above, or explore a{' '}
            <a href={`/share/${popularDomains[0]?.share_id}`} className="text-primary-400 hover:text-primary-300 transition-colors">
              sample report
            </a>{' '}
            to see what you'll get
          </p>
        )}

        {/* Alert Banner — shown on all tabs when models are degraded */}
        {healthHistory.length > 0 && healthHistory[0].models.some(m => m.status !== 'available') && (
          <div className="max-w-2xl mx-auto mb-6 animate-fade-in">
            <div className={`border rounded-xl p-4 flex items-start gap-3 ${
              healthHistory[0].models.every(m => m.status !== 'available')
                ? 'bg-red-500/10 border-red-500/20'
                : 'bg-amber-500/10 border-amber-500/20'
            }`}>
              <svg className={`w-5 h-5 shrink-0 mt-0.5 ${
                healthHistory[0].models.every(m => m.status !== 'available') ? 'text-red-400' : 'text-amber-400'
              }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
              </svg>
              <p className={`text-sm ${
                healthHistory[0].models.every(m => m.status !== 'available') ? 'text-red-300' : 'text-amber-300'
              }`}>
                {healthHistory[0].models.every(m => m.status !== 'available')
                  ? 'All API providers are currently unavailable. Analysis requests will fail until service is restored.'
                  : (() => {
                      const degraded = healthHistory[0].models.filter(m => m.status !== 'available')
                      return `${degraded.map(m => m.name).join(', ')} ${degraded.length > 1 ? 'are' : 'is'} currently unavailable.`
                    })()
                }
              </p>
            </div>
          </div>
        )}

        {/* ===== ANALYZE TAB ===== */}
        {activeTab === 'analyze' && (
          <>
            {/* Popular Brands */}
            {state !== 'done' && popularDomains.length > 0 && (
              <div className="max-w-4xl mx-auto mb-12 animate-fade-in">
                <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4 text-center">
                  Popular Brands
                </h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {popularDomains.map(pd => (
                    <a
                      key={pd.share_id}
                      href={`/share/${pd.share_id}`}
                      className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl overflow-hidden hover:border-primary-500/50 transition-all cursor-pointer group block"
                    >
                      {pd.has_screenshot && (
                        <div className="w-full h-48 bg-dark-800 overflow-hidden">
                          <img
                            src={`/api/share/popular/${encodeURIComponent(pd.domain)}/screenshot`}
                            alt={`${pd.brand_name || pd.domain} homepage`}
                            className="w-full h-full object-cover object-top opacity-70 group-hover:opacity-90 transition-opacity"
                            loading="lazy"
                          />
                        </div>
                      )}
                      <div className="p-4">
                        <div className="flex items-center gap-3">
                          {pd.avg_score > 0 ? (
                            <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-sm font-bold shrink-0 border ${scoreBadge(pd.avg_score)}`}>
                              {pd.avg_score}
                            </div>
                          ) : (
                            <div className="w-10 h-10 rounded-lg flex items-center justify-center text-sm shrink-0 border bg-dark-800/50 text-dark-500 border-dark-700">
                              --
                            </div>
                          )}
                          <div className="flex-1 min-w-0">
                            <p className="text-white text-sm font-medium group-hover:text-primary-300 transition-colors truncate">
                              {pd.brand_name || pd.domain}
                            </p>
                            <p className="text-dark-500 text-xs">
                              {pd.domain}
                              {pd.report_count > 0 && <> · {pd.report_count} {pd.report_count === 1 ? 'report' : 'reports'}</>}
                              {pd.has_video && <> · video</>}
                            </p>
                          </div>
                          <svg className="w-4 h-4 text-dark-600 group-hover:text-primary-400 transition-colors shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
                          </svg>
                        </div>
                      </div>
                    </a>
                  ))}
                </div>
              </div>
            )}

            {/* Progress */}
            {state === 'analyzing' && statusMessages.length > 0 && (
              <div className="max-w-2xl mx-auto mb-12 animate-fade-in">
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-3">
                    {statusMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {i < statusMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                          </svg>
                        ) : (
                          <div className="w-4 h-4 shrink-0 flex items-center justify-center">
                            <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                          </div>
                        )}
                        <span className={i < statusMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>
                          {msg}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}

            {/* Error */}
            {state === 'error' && (
              <div className="max-w-2xl mx-auto mb-12 animate-fade-in">
                <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-4">
                  <p className="text-red-400 text-sm">{error}</p>
                </div>
              </div>
            )}

            {/* Results */}
            {result && (
              <div className="space-y-8 animate-slide-up">

                {/* Dashboard Scorecard */}
                {visibilityScore && visibilityScore.available > 0 && (
                  <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                    <div className="flex items-center gap-6 mb-4">
                      <div className={`w-20 h-20 rounded-2xl border-2 flex flex-col items-center justify-center ${
                        visibilityScore.score >= 70 ? 'border-emerald-500/40 bg-emerald-500/10' : visibilityScore.score >= 40 ? 'border-amber-500/40 bg-amber-500/10' : 'border-red-500/40 bg-red-500/10'
                      }`}>
                        <span className={`text-3xl font-bold ${scoreTextColor(visibilityScore.score)}`}>{visibilityScore.score}</span>
                        <span className="text-dark-500 text-[10px]">/100</span>
                      </div>
                      <div>
                        <h3 className="text-white font-semibold text-lg">LLM Visibility Score</h3>
                        <p className="text-dark-400 text-sm">{scoreLabel(visibilityScore.score)} &middot; Based on {visibilityScore.available} of {visibilityScore.components.length} dimensions</p>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
                      {visibilityScore.components.map(c => {
                        const tabMap: Record<string, string> = { 'Optimization': 'optimize', 'Video Authority': 'video', 'Reddit Authority': 'reddit', 'Search Visibility': 'search', 'LLM Test': 'test' }
                        return (
                          <button
                            key={c.name}
                            onClick={() => { if (tabMap[c.name]) setActiveTab(tabMap[c.name] as typeof activeTab) }}
                            className={`rounded-xl p-3 text-center transition-all cursor-pointer group ${
                              c.available
                                ? 'bg-dark-800/50 border border-dark-700 hover:border-primary-500/30'
                                : 'bg-dark-900/30 border border-dashed border-dark-600 hover:border-primary-500/30'
                            }`}
                          >
                            {c.available ? (
                              <>
                                <div className={`text-xl font-bold ${scoreTextColor(c.score)}`}>{c.score}</div>
                                <div className="text-dark-400 text-[10px] mt-0.5">{c.name}</div>
                                <div className="text-dark-600 text-[9px]">{Math.round(c.weight * 100)}% weight</div>
                              </>
                            ) : (
                              <>
                                <div className="text-dark-500 text-xs font-medium">Not yet</div>
                                <div className="text-dark-400 text-[10px] mt-0.5">{c.name}</div>
                                <div className="text-primary-400/70 text-[9px] mt-0.5 group-hover:text-primary-400 transition-colors">Run analysis</div>
                              </>
                            )}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )}

                {/* Result metadata bar */}
                {resultMeta && (
                  <div className="flex items-center justify-between flex-wrap gap-3">
                    <div className="flex items-center gap-3 text-sm text-dark-400">
                      {resultMeta.createdAt && (
                        <span>
                          Analyzed {new Date(resultMeta.createdAt).toLocaleDateString(undefined, {
                            month: 'short', day: 'numeric', year: 'numeric',
                            hour: '2-digit', minute: '2-digit',
                          })}
                        </span>
                      )}
                      {resultMeta.model && (
                        <>
                          <span className="text-dark-600">·</span>
                          <span>using {resultMeta.model}</span>
                        </>
                      )}
                      {resultMeta.cached && (
                        <span className="text-xs px-2 py-0.5 rounded-full bg-primary-500/15 text-primary-400 border border-primary-500/20 font-medium">
                          Cached result
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      {!readOnly && (
                        <button
                          onClick={() => analyze(true)}
                          disabled={state === 'analyzing'}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                        >
                          Re-analyze
                        </button>
                      )}
                    </div>
                  </div>
                )}

                {/* Brand Intelligence Indicator */}
                {resultMeta && (() => {
                  const domain = url.trim()
                  const brandEntry = brandList.find(b => b.domain === domain)
                  const brandContentTs = brandEntry?.brand_content_updated_at || brandEntry?.updated_at
                  const brandNewer = brandEntry && resultMeta.brandProfileUpdatedAt && brandContentTs && new Date(brandContentTs) > new Date(resultMeta.brandProfileUpdatedAt)
                  const noBrandUsed = !resultMeta.brandContextUsed
                  if (resultMeta.brandContextUsed && !brandNewer) {
                    return (
                      <div className="flex items-center gap-2 px-4 py-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-xl">
                        <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
                        <span className="text-emerald-400 text-xs font-medium">Brand Intelligence Active</span>
                      </div>
                    )
                  }
                  if (brandEntry && (noBrandUsed || brandNewer)) {
                    return (
                      <div className="flex items-center justify-between gap-3 px-4 py-2.5 bg-amber-500/10 border border-amber-500/20 rounded-xl">
                        <div className="flex items-center gap-2">
                          <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                          <span className="text-amber-400 text-xs font-medium">
                            {brandNewer ? 'Brand Intelligence has been updated since this analysis' : 'Brand Intelligence available but not used in this analysis'}
                          </span>
                        </div>
                        {!readOnly && (
                          <button
                            onClick={() => analyze(true)}
                            disabled={state === 'analyzing'}
                            className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap shrink-0"
                          >
                            Re-run with Brand Intel
                          </button>
                        )}
                      </div>
                    )
                  }
                  if (!brandEntry && !readOnly) {
                    return (
                      <div className="flex items-center justify-between gap-3 px-4 py-2.5 bg-dark-900/50 border border-dark-800 rounded-xl">
                        <span className="text-dark-500 text-xs">No Brand Intelligence set up for this domain</span>
                        <button
                          onClick={() => { startNewBrand(domain); setActiveTab('brand') }}
                          className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap shrink-0"
                        >
                          Set up Brand Intel
                        </button>
                      </div>
                    )
                  }
                  return null
                })()}

                {/* Domain Executive Summary */}
                {generatingSummary ? (
                  <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">Generating Executive Summary</h3>
                    {summaryMessages.length > 0 && (
                      <div className="space-y-3">
                        {summaryMessages.map((msg, i) => (
                          <div key={i} className="flex items-center gap-3 text-sm">
                            {i < summaryMessages.length - 1 ? (
                              <svg className="w-4 h-4 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                                <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                              </svg>
                            ) : (
                              <div className="w-4 h-4 shrink-0 flex items-center justify-center">
                                <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                              </div>
                            )}
                            <span className={i < summaryMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>{msg}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ) : activeSummary ? (
                  <div className="space-y-4">
                    {/* Summary header */}
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                      <div className="flex items-center justify-between mb-3">
                        <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">Executive Summary</h3>
                        {!readOnly && (
                          <button
                            onClick={() => generateSummary(selectedDomain)}
                            className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer"
                          >
                            Regenerate
                          </button>
                        )}
                      </div>

                      {/* Stale banner */}
                      {activeSummaryStale && !readOnly && (
                        <div className="flex items-center justify-between bg-amber-500/10 border border-amber-500/20 rounded-xl px-4 py-3 mb-4">
                          <span className="text-amber-400 text-xs">New or updated reports available since this summary was generated.</span>
                          <button
                            onClick={() => generateSummary(selectedDomain)}
                            className="text-xs px-3 py-1.5 bg-amber-500/20 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/30 transition-all cursor-pointer whitespace-nowrap ml-3"
                          >
                            Regenerate
                          </button>
                        </div>
                      )}

                      <div className="mb-4">
                        <div className={`text-dark-300 text-sm leading-relaxed whitespace-pre-line ${!expandedSections.has('domain-summary') ? 'line-clamp-3' : ''}`}>
                          {activeSummary.result.executive_summary}
                        </div>
                        {activeSummary.result.executive_summary.split('\n').length > 3 && (
                          <button
                            onClick={() => setExpandedSections(prev => {
                              const next = new Set(prev)
                              next.has('domain-summary') ? next.delete('domain-summary') : next.add('domain-summary')
                              return next
                            })}
                            className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer mt-1"
                          >
                            {expandedSections.has('domain-summary') ? 'Show less' : 'Show more'}
                          </button>
                        )}
                      </div>

                      {/* Score + dimension bars */}
                      {activeSummary.result.average_score > 0 && (
                        <div className="flex items-start gap-4 pt-4 border-t border-dark-800">
                          <div className={`w-12 h-12 rounded-xl flex items-center justify-center text-lg font-bold border shrink-0 ${scoreBadge(activeSummary.result.average_score)}`}>
                            {activeSummary.result.average_score}
                          </div>
                          <div className="flex-1">
                            <div className="text-dark-500 text-xs mb-2">Avg Score (range {activeSummary.result.score_range[0]}–{activeSummary.result.score_range[1]})</div>
                            {activeSummary.result.dimension_trends && (
                              <div className="grid grid-cols-2 gap-2">
                                {[
                                  { key: 'content_authority', label: 'Content Authority' },
                                  { key: 'structural_optimization', label: 'Structural' },
                                  { key: 'source_authority', label: 'Source Authority' },
                                  { key: 'knowledge_persistence', label: 'Persistence' },
                                ].map(dim => {
                                  const score = activeSummary.result.dimension_trends[dim.key] ?? 0
                                  return (
                                    <div key={dim.key} className="flex items-center gap-2">
                                      <span className="text-dark-500 text-xs w-20 shrink-0">{dim.label}</span>
                                      <div className="flex-1 h-1.5 bg-dark-700 rounded-full overflow-hidden">
                                        <div className={`h-full rounded-full ${score >= 70 ? 'bg-emerald-500' : score >= 50 ? 'bg-amber-500' : 'bg-red-500'}`} style={{ width: `${score}%` }} />
                                      </div>
                                      <span className={`text-xs font-bold w-6 text-right ${score >= 70 ? 'text-emerald-400' : score >= 50 ? 'text-amber-400' : 'text-red-400'}`}>{score}</span>
                                    </div>
                                  )
                                })}
                              </div>
                            )}
                          </div>
                        </div>
                      )}

                      {/* Metadata */}
                      <div className="flex items-center gap-3 text-xs text-dark-600 mt-3">
                        <span>{[
                          activeSummary.report_count > 0 ? `${activeSummary.report_count} optimization report${activeSummary.report_count !== 1 ? 's' : ''}` : null,
                          activeSummary.includes_analysis ? 'Site Analysis' : null,
                          activeSummary.includes_video ? 'YouTube' : null,
                          activeSummary.includes_reddit ? 'Reddit' : null,
                        ].filter(Boolean).join(' + ') || 'No reports'} analyzed</span>
                        <span>·</span>
                        <span>{new Date(activeSummary.generated_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}</span>
                      </div>
                    </div>

                    {/* Key Themes */}
                    {activeSummary.result.themes?.length > 0 && (
                      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                        <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">Key Themes</h3>
                        <div className="space-y-3">
                          {activeSummary.result.themes.map((theme, i) => (
                            <div key={i} className="bg-dark-800/50 rounded-xl p-4">
                              <h4 className="text-white font-medium text-sm mb-1">{theme.title}</h4>
                              <p className="text-dark-400 text-sm leading-relaxed">{theme.description}</p>
                              {theme.report_refs?.length > 0 && (
                                <div className="mt-2 text-xs text-dark-500">From: {theme.report_refs.join(', ')}</div>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                  </div>
                ) : !readOnly ? (
                  <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 flex items-center justify-between">
                    <div>
                      <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-1">Executive Summary</h3>
                      <p className="text-dark-500 text-xs">Synthesize all reports for this domain into a strategic overview.</p>
                    </div>
                    <button
                      onClick={() => generateSummary(selectedDomain)}
                      className="text-xs px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer whitespace-nowrap"
                    >
                      Generate Summary
                    </button>
                  </div>
                ) : null}

                {/* Site Summary Card */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                  <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-3">
                    Site Summary
                  </h3>
                  <p className="text-dark-200 text-lg leading-relaxed">{result.site_summary}</p>
                </div>

                {/* Questions */}
                <div>
                  <div className="flex items-center justify-between mb-5">
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                      Questions People Ask ({result.questions.length})
                    </h3>
                    <div className="flex gap-2 flex-wrap justify-end">
                      {categories.map(cat => (
                        <span
                          key={cat}
                          className={`text-xs px-2 py-0.5 rounded-full border font-medium ${categoryColors[cat]}`}
                        >
                          {cat}
                        </span>
                      ))}
                    </div>
                  </div>

                  {!readOnly && (
                    <div className="flex items-center justify-between gap-3">
                      <label className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={optimizeAutoArchive}
                          onChange={e => setOptimizeAutoArchive(e.target.checked)}
                          className="w-3.5 h-3.5 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500/30 cursor-pointer"
                        />
                        <span className="text-dark-400 text-xs">Automatically archive incomplete optimization recommendations when re-running a question</span>
                      </label>
                      {resultMeta?.id && result.questions.some((q) => !optList.some(o => domainKey(o.domain) === domainKey(selectedDomain) && o.question === q.question)) && (
                        <button
                          onClick={batchOptimize}
                          disabled={batchOptimizing || optimizing}
                          className="text-xs px-3 py-1.5 rounded-lg transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap shrink-0 bg-accent-purple/10 border border-accent-purple/20 text-purple-300 hover:bg-accent-purple/20 hover:text-purple-200"
                        >
                          {batchOptimizing
                            ? `Optimizing ${batchOptProgress.current}/${batchOptProgress.total}...`
                            : 'Optimize All'}
                        </button>
                      )}
                    </div>
                  )}

                  {/* Discovery questions callout */}
                  {result.questions.some((q: any) => q.brand_status === 'discovery') && (
                    <div className="bg-teal-500/5 border border-teal-500/20 rounded-xl px-5 py-3 mb-4">
                      <p className="text-dark-400 text-xs leading-relaxed">
                        <span className="text-teal-400 font-medium">Discovery Questions:</span>{' '}
                        Questions marked "Discovery" represent what your target audience searches for without knowing your brand — category-level queries like "best tools for X" or "how to do Y." Optimizing for these helps new customers find you for the first time.
                      </p>
                    </div>
                  )}

                  <div className="grid gap-3">
                    {result.questions.map((q, i) => (
                      <div
                        key={i}
                        className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5 hover:border-dark-700 transition-colors group"
                      >
                        <div className="flex items-start justify-between gap-4 mb-2">
                          <p className="text-white font-medium leading-snug group-hover:text-primary-300 transition-colors">
                            {q.question}
                          </p>
                          <div className="flex items-center gap-2 shrink-0">
                            {optScores[i] !== undefined && (
                              <span className={`text-xs px-1.5 py-0.5 rounded font-semibold ${scoreBadge(optScores[i])}`}>
                                {optScores[i]}
                              </span>
                            )}
                            {q.brand_status === 'aspirational' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-amber-500/20 text-amber-400 border border-amber-500/30">
                                Aspirational
                              </span>
                            )}
                            {q.brand_status === 'missing' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-rose-500/20 text-rose-400 border border-rose-500/30">
                                Missing?
                              </span>
                            )}
                            {q.brand_status === 'discovery' && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-teal-500/20 text-teal-400 border border-teal-500/30">
                                Discovery
                              </span>
                            )}
                            <span
                              className={`text-xs px-2 py-0.5 rounded-full border font-medium whitespace-nowrap ${
                                categoryColors[q.category] || 'bg-dark-700/50 text-dark-400 border-dark-600'
                              }`}
                            >
                              {q.category}
                            </span>
                          </div>
                        </div>
                        <p className="text-dark-400 text-sm leading-relaxed">{q.relevance}</p>
                        <div className="mt-2.5 flex items-center justify-between gap-3">
                          <div className="flex flex-wrap gap-1.5">
                            {q.page_urls && q.page_urls.map((pageUrl, j) => (
                              <a
                                key={j}
                                href={pageUrl}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-xs text-primary-400 hover:text-primary-300 bg-primary-500/10 border border-primary-500/20 rounded-full px-2.5 py-0.5 transition-colors"
                              >
                                {safePathname(pageUrl)}
                              </a>
                            ))}
                          </div>
                          {resultMeta?.id && (
                            <button
                              onClick={() => handleQuestionClick(i)}
                              disabled={optimizing}
                              className={`text-xs px-3 py-1 rounded-lg transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap shrink-0 ${
                                readOnly
                                  ? optScores[i] !== undefined
                                    ? 'bg-primary-500/10 border border-primary-500/20 text-primary-300 hover:bg-primary-500/20'
                                    : 'bg-dark-800 border border-dark-700 text-dark-400 hover:bg-dark-700'
                                  : 'bg-accent-purple/10 border border-accent-purple/20 text-purple-300 hover:bg-accent-purple/20 hover:text-purple-200'
                              }`}
                            >
                              {optimizing && optimizationMeta?.questionIndex === i
                                ? 'Optimizing...'
                                : readOnly
                                  ? optScores[i] !== undefined ? 'View Report' : 'View'
                                  : optScores[i] !== undefined ? 'View Report' : 'Optimize'}
                            </button>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Crawled Pages */}
                {result.crawled_pages && result.crawled_pages.length > 0 && (
                  <div>
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                      Pages Crawled ({result.crawled_pages.length})
                    </h3>
                    <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                      <div className="space-y-2">
                        {result.crawled_pages.map((page, i) => (
                          <div key={i} className="flex items-center gap-3">
                            <a
                              href={page.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-sm text-primary-400 hover:text-primary-300 transition-colors truncate"
                            >
                              {page.url}
                            </a>
                            {page.title && (
                              <span className="text-dark-500 text-xs shrink-0">
                                {page.title}
                              </span>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
        )}

        {/* ===== OPTIMIZE TAB ===== */}
        {activeTab === 'optimize' && (
          <div className="max-w-2xl mx-auto animate-fade-in space-y-6">

            {/* Cross-report insights */}
            {crossInsights.filter(i => i.tab === 'optimize' && !dismissedInsights.has(i.message)).map(insight => (
              <div key={insight.message} className="flex items-center justify-between bg-primary-500/5 border border-primary-500/20 rounded-xl px-4 py-3">
                <span className="text-dark-300 text-xs flex-1 mr-3">{insight.message}</span>
                <div className="flex items-center gap-2 shrink-0">
                  <button onClick={() => setActiveTab(insight.targetTab as typeof activeTab)} className="text-xs px-3 py-1 bg-primary-500/10 border border-primary-500/20 text-primary-400 rounded-lg hover:bg-primary-500/20 transition-all cursor-pointer">{insight.cta}</button>
                  <button onClick={() => setDismissedInsights(prev => new Set([...prev, insight.message]))} className="text-dark-600 hover:text-dark-400 cursor-pointer"><svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg></button>
                </div>
              </div>
            ))}

            {/* === Running view: SSE progress === */}
            {optimizeView === 'running' && (
              <>
                {optimizing && optimizeMessages.length > 0 && (
                  <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                        Analyzing LLM Visibility
                      </h3>
                      <button
                        onClick={cancelOptimize}
                        className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer"
                      >
                        Cancel
                      </button>
                    </div>
                    <div className="space-y-3">
                      {optimizeMessages.map((msg, i) => (
                        <div key={i} className="flex items-center gap-3 text-sm">
                          {i < optimizeMessages.length - 1 ? (
                            <svg className="w-4 h-4 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                            </svg>
                          ) : (
                            <div className="w-4 h-4 shrink-0 flex items-center justify-center">
                              <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                            </div>
                          )}
                          <span className={i < optimizeMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>
                            {msg}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {optimizeError && (
                  <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-4">
                    <p className="text-red-400 text-sm">{optimizeError}</p>
                    <button
                      onClick={() => setOptimizeView('list')}
                      className="text-xs text-dark-400 hover:text-white mt-2 cursor-pointer"
                    >
                      Back to reports
                    </button>
                  </div>
                )}
              </>
            )}

            {/* === List view: past optimization reports === */}
            {optimizeView === 'list' && (
              <>
                {optListLoading ? (
                  <div className="flex justify-center py-16">
                    <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                  </div>
                ) : (() => {
                  const filtered = optList.filter(o => domainKey(o.domain) === domainKey(selectedDomain))
                  const avgScore = filtered.length > 0 ? Math.round(filtered.reduce((s, o) => s + o.overall_score, 0) / filtered.length) : 0
                  return filtered.length === 0 ? (
                    <div className="text-center py-16 max-w-md mx-auto">
                      <svg className="w-12 h-12 mx-auto text-dark-600 mb-4" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 3v11.25A2.25 2.25 0 006 16.5h2.25M3.75 3h-1.5m1.5 0h16.5m0 0h1.5m-1.5 0v11.25A2.25 2.25 0 0118 16.5h-2.25m-7.5 0h7.5m-7.5 0l-1 3m8.5-3l1 3m0 0l.5 1.5m-.5-1.5h-9.5m0 0l-.5 1.5M9 11.25v1.5M12 9v3.75m3-6v6" /></svg>
                      <h3 className="text-white font-semibold text-lg mb-2">Get Your First Optimization Report</h3>
                      <p className="text-dark-400 text-sm mb-4">See exactly what to fix so AI search engines recommend your brand. Each report scores your content across 4 research-backed dimensions and gives you a prioritized action plan.</p>
                      <button onClick={() => setActiveTab('analyze')} className="text-sm px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-500 transition-colors cursor-pointer">Discover Questions First</button>
                    </div>
                  ) : (
                    <div className="space-y-4">
                      {/* Domain header */}
                      <div className="flex items-center gap-3">
                        <div className={`w-10 h-10 rounded-xl flex items-center justify-center text-sm font-bold shrink-0 border ${scoreBadge(avgScore)}`}>
                          {avgScore}
                        </div>
                        <div>
                          <h4 className="text-white font-medium text-sm">{selectedDomain}</h4>
                          <span className="text-xs text-dark-500">{filtered.length} {filtered.length === 1 ? 'report' : 'reports'}</span>
                        </div>
                      </div>
                      {/* Report cards */}
                      <div className="space-y-2">
                        {filtered.map(item => (
                          <button
                            key={item.id}
                            onClick={() => loadOptimizationDetail(item.id)}
                            className="w-full text-left bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl p-4 hover:border-primary-500/40 transition-all cursor-pointer group"
                          >
                            <div className="flex items-start gap-3">
                              <div className={`w-9 h-9 rounded-lg flex items-center justify-center text-sm font-bold shrink-0 ${scoreBadge(item.overall_score)}`}>
                                {item.overall_score}
                              </div>
                              <div className="flex-1 min-w-0">
                                <p className="text-white text-sm leading-snug group-hover:text-primary-300 transition-colors line-clamp-2">
                                  {item.question}
                                </p>
                                <div className="flex items-center gap-2 mt-1 text-xs text-dark-500">
                                  <span>{fmtDate(item.created_at)}</span>
                                  {item.model && (
                                    <>
                                      <span className="text-dark-600">·</span>
                                      <span>via {item.model}</span>
                                    </>
                                  )}
                                  {item.brand_context_used ? (
                                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-400 border border-emerald-500/20 font-medium">Brand Intel</span>
                                  ) : (
                                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-dark-700/50 text-dark-500 border border-dark-600 font-medium">No Brand Intel</span>
                                  )}
                                </div>
                              </div>
                            </div>
                          </button>
                        ))}
                      </div>
                    </div>
                  )
                })()}
              </>
            )}

            {/* === Detail view: full optimization report === */}
            {optimizeView === 'detail' && optimization && optimizationMeta && (
              <>
                {/* Back button */}
                <button
                  onClick={() => { setOptimizeView('list'); fetchOptList() }}
                  className="text-xs text-dark-400 hover:text-white transition-colors cursor-pointer flex items-center gap-1"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                  </svg>
                  All reports
                </button>

                {/* Header */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                      LLM Visibility Analysis
                    </h3>
                    <div className="flex items-center gap-2">
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      {!readOnly && resultMeta?.id && (
                        <button
                          onClick={() => optimizeQuestion(optimizationMeta.questionIndex, true)}
                          disabled={optimizing}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                        >
                          Re-analyze
                        </button>
                      )}
                      {!readOnly && (
                        <button
                          onClick={() => setConfirmDeleteOptimization(optimizationMeta.id)}
                          className="text-xs px-3 py-1.5 text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-all cursor-pointer"
                        >
                          Delete
                        </button>
                      )}
                    </div>
                  </div>
                  <p className="text-white font-medium text-lg leading-snug mb-2">
                    {optimizationMeta.question}
                    {optimizationMeta.brandStatus === 'aspirational' && (
                      <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-amber-500/20 text-amber-400 border border-amber-500/30 align-middle">Aspirational</span>
                    )}
                    {optimizationMeta.brandStatus === 'missing' && (
                      <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-rose-500/20 text-rose-400 border border-rose-500/30 align-middle">Missing?</span>
                    )}
                    {optimizationMeta.brandStatus === 'discovery' && (
                      <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-teal-500/20 text-teal-400 border border-teal-500/30 align-middle">Discovery</span>
                    )}
                  </p>
                  <div className="flex items-center gap-3 text-sm text-dark-400 flex-wrap">
                    <span>{selectedOpt?.domain || url}</span>
                    {optimizationMeta.createdAt && (
                      <>
                        <span className="text-dark-600">·</span>
                        <span>{new Date(optimizationMeta.createdAt).toLocaleDateString(undefined, {
                          month: 'short', day: 'numeric', year: 'numeric',
                          hour: '2-digit', minute: '2-digit',
                        })}</span>
                      </>
                    )}
                    {optimizationMeta.model && (
                      <>
                        <span className="text-dark-600">·</span>
                        <span>via {optimizationMeta.model}</span>
                      </>
                    )}
                    {optimizationMeta.cached && (
                      <span className="text-xs px-2 py-0.5 rounded-full bg-primary-500/15 text-primary-400 border border-primary-500/20 font-medium">
                        Cached
                      </span>
                    )}
                  </div>
                </div>

                {/* Brand Intelligence Warning */}
                {(() => {
                  const domain = selectedOpt?.domain || url
                  const brandEntry = brandList.find(b => b.domain === domain)
                  const brandContentTs = brandEntry?.brand_content_updated_at || brandEntry?.updated_at
                  const brandNewer = brandEntry && optimizationMeta.brandProfileUpdatedAt && brandContentTs && new Date(brandContentTs) > new Date(optimizationMeta.brandProfileUpdatedAt)
                  if (optimizationMeta.brandContextUsed && !brandNewer) {
                    return (
                      <div className="flex items-center gap-2 px-4 py-2.5 bg-emerald-500/10 border border-emerald-500/20 rounded-xl">
                        <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
                        <span className="text-emerald-400 text-xs font-medium">Brand Intelligence Active</span>
                      </div>
                    )
                  }
                  if (!optimizationMeta.brandContextUsed || brandNewer) {
                    return (
                      <div className="flex items-center justify-between gap-3 px-4 py-2.5 bg-amber-500/10 border border-amber-500/20 rounded-xl">
                        <div className="flex items-center gap-2 min-w-0">
                          <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                          <span className="text-amber-400 text-xs font-medium">
                            {brandNewer ? 'Brand Intelligence has been updated since this report was generated' : 'This report was generated without Brand Intelligence'}
                          </span>
                        </div>
                        {!readOnly && (
                          <div className="flex items-center gap-2 shrink-0">
                            {!brandEntry && (
                              <button
                                onClick={() => { startNewBrand(domain); setActiveTab('brand') }}
                                className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap"
                              >
                                Set up Brand Intel
                              </button>
                            )}
                            {brandEntry && (
                              <button
                                onClick={() => { loadBrandProfile(domain); setActiveTab('brand') }}
                                className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap"
                              >
                                View Brand Intel
                              </button>
                            )}
                            {brandEntry && resultMeta?.id && (
                              <button
                                onClick={() => optimizeQuestion(optimizationMeta.questionIndex, true)}
                                disabled={optimizing}
                                className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all disabled:opacity-50 cursor-pointer whitespace-nowrap"
                              >
                                Re-run with Brand Intel
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    )
                  }
                  return null
                })()}

                {/* Overall Score */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 flex items-center gap-6">
                  <div className={`w-20 h-20 rounded-2xl flex items-center justify-center text-2xl font-bold shrink-0 border ${scoreBadge(optimization.overall_score)}`}>
                    {optimization.overall_score}
                  </div>
                  <div>
                    <h3 className="text-white font-semibold text-lg mb-1">
                      {scoreLabel(optimization.overall_score)}
                    </h3>
                    <p className="text-dark-400 text-sm">
                      Overall LLM Visibility Score based on content authority, structure, source authority, and knowledge persistence.
                    </p>
                  </div>
                </div>

                {/* Dimension Cards */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {([
                    { key: 'content_authority' as const, name: 'Content Authority', weight: '30%' },
                    { key: 'structural_optimization' as const, name: 'Structural Optimization', weight: '20%' },
                    { key: 'source_authority' as const, name: 'Source Authority', weight: '30%' },
                    { key: 'knowledge_persistence' as const, name: 'Knowledge Persistence', weight: '20%' },
                  ] as const).map(dim => {
                    const d = optimization[dim.key]
                    return (
                      <div key={dim.key} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                        <div className="flex items-center justify-between mb-3">
                          <div>
                            <h4 className="text-white font-medium text-sm">{dim.name}</h4>
                            <span className="text-dark-500 text-xs">{dim.weight} weight</span>
                          </div>
                          <span className={`text-lg font-bold ${scoreTextColor(d.score)}`}>{d.score}</span>
                        </div>
                        <div className="h-1.5 bg-dark-800 rounded-full mb-4 overflow-hidden">
                          <div
                            className={`h-full rounded-full transition-all ${scoreBgSolid(d.score)}`}
                            style={{ width: `${d.score}%` }}
                          />
                        </div>
                        {d.evidence && d.evidence.length > 0 && (
                          <div className="mb-3">
                            <p className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-1.5">Evidence</p>
                            <ul className="space-y-1">
                              {d.evidence.map((e, j) => (
                                <li key={j} className="text-dark-300 text-xs leading-relaxed flex gap-2">
                                  <span className="text-dark-600 shrink-0">-</span>
                                  <span>{e}</span>
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                        {d.improvements && d.improvements.length > 0 && (
                          <div>
                            <p className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-1.5">Improvements</p>
                            <ul className="space-y-1">
                              {d.improvements.map((imp, j) => (
                                <li key={j} className="text-primary-400 text-xs leading-relaxed flex gap-2">
                                  <span className="text-primary-600 shrink-0">+</span>
                                  <span>{imp}</span>
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>

                {/* Competitors */}
                {optimization.competitors && optimization.competitors.length > 0 && (
                  <div>
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                      Competing Sources ({optimization.competitors.length})
                    </h3>
                    <div className="grid gap-3">
                      {optimization.competitors.map((comp, i) => (
                        <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4 flex items-center gap-4">
                          <span className={`text-lg font-bold shrink-0 w-10 text-center ${scoreTextColor(comp.score_estimate)}`}>{comp.score_estimate}</span>
                          <div className="min-w-0">
                            <p className="text-white font-medium text-sm truncate">{comp.domain}</p>
                            <p className="text-dark-400 text-xs leading-relaxed">{comp.strengths}</p>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Recommendations with todo status */}
                {optimization.recommendations && optimization.recommendations.length > 0 && (
                  <div>
                    <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                      Recommendations
                    </h3>
                    <div className="space-y-3">
                      {optimization.recommendations.map((rec, i) => {
                        const matchingTodo = optTodos.find(t => t.action === rec.action)
                        return (
                          <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                            <div className="flex items-center gap-2 mb-2">
                              <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                rec.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                rec.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                'bg-dark-700/50 text-dark-400 border border-dark-600'
                              }`}>
                                {rec.priority}
                              </span>
                              <span className="text-dark-500 text-xs capitalize">{rec.dimension.replace(/_/g, ' ')}</span>
                              {matchingTodo && (
                                <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ml-auto ${
                                  matchingTodo.status === 'completed' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                                  matchingTodo.status === 'backlogged' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                  'bg-primary-500/20 text-primary-400 border border-primary-500/30'
                                }`}>
                                  {matchingTodo.status === 'todo' ? 'new' : matchingTodo.status}
                                </span>
                              )}
                            </div>
                            <p className="text-white text-sm font-medium mb-1">{rec.action}</p>
                            <p className="text-dark-400 text-xs">{rec.expected_impact}</p>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* ===== TODOS TAB ===== */}
        {activeTab === 'todos' && (
          <div className="max-w-2xl mx-auto animate-fade-in space-y-6">
            {/* Sub-tabs + sort toggle */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="inline-flex bg-dark-900/50 border border-dark-800 rounded-lg p-1">
                  {(['todo', 'completed', 'backlogged', 'archived'] as const).map(st => (
                    <button
                      key={st}
                      onClick={() => setTodoSubTab(st)}
                      className={`px-4 py-1.5 rounded-md text-xs font-medium transition-all cursor-pointer capitalize ${
                        todoSubTab === st
                          ? 'bg-dark-700 text-white'
                          : 'text-dark-400 hover:text-white'
                      }`}
                    >
                      {st === 'todo' ? 'To-Do' : st === 'completed' ? 'Completed' : st === 'backlogged' ? 'Backlogged' : 'Archived'}
                      <span className="ml-1.5 text-dark-500">
                        {todos.filter(t => t.status === st && domainKey(t.domain) === domainKey(selectedDomain)).length}
                      </span>
                    </button>
                  ))}
                </div>
                <div className="flex items-center gap-2">
                  {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                    <button
                      onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                      className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                      title="Share this domain"
                    >
                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                      Share
                    </button>
                  )}
                  <button
                    onClick={fetchTodos}
                    disabled={todosLoading}
                    className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                  >
                  {todosLoading ? 'Loading...' : 'Refresh'}
                  </button>
                </div>
              </div>
              {/* Sort mode toggle */}
              <div className="flex items-center gap-2">
                <span className="text-dark-500 text-xs">Sort by</span>
                <div className="inline-flex bg-dark-900/50 border border-dark-800 rounded-md p-0.5">
                  {(['priority', 'question', 'dimension'] as const).map(mode => (
                    <button
                      key={mode}
                      onClick={() => setTodoSortMode(mode)}
                      className={`px-3 py-1 rounded text-xs font-medium transition-all cursor-pointer capitalize ${
                        todoSortMode === mode
                          ? 'bg-dark-700 text-white'
                          : 'text-dark-500 hover:text-white'
                      }`}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Brand Intel virtual todo for selected domain */}
            {(() => {
              const brand = brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))
              // In shared mode, use brandCompleteness from form state since brandList isn't populated
              const completeness = sharedMode
                ? (brandEditing ? brandCompleteness : 0)
                : (brand?.completeness ?? 0)
              const brandName = brand?.brand_name || (sharedMode ? (brandForm.brand_name || selectedDomain) : selectedDomain)
              const brandIntelItems = [{ domain: selectedDomain, completeness, brandName }]

              // Items with <100% go in "todo", items with 100% go in "completed"
              const incompleteBrandItems = brandIntelItems.filter(b => b.completeness < 100)
              const completeBrandItems = brandIntelItems.filter(b => b.completeness === 100)

              return (
                <>
                  {/* Sub-tab counts with brand intel virtual items */}
                  <div className="flex items-center gap-2 text-xs text-dark-500 -mt-2 mb-2">
                    {incompleteBrandItems.length > 0 && todoSubTab === 'todo' && (
                      <span className="text-amber-400">Includes {incompleteBrandItems.length} Brand Intelligence {incompleteBrandItems.length === 1 ? 'item' : 'items'}</span>
                    )}
                    {completeBrandItems.length > 0 && todoSubTab === 'completed' && (
                      <span className="text-emerald-400">{completeBrandItems.length} Brand Intelligence {completeBrandItems.length === 1 ? 'profile' : 'profiles'} at 100%</span>
                    )}
                  </div>

                  {/* Brand Intel Virtual Todo Items */}
                  {todoSubTab === 'todo' && incompleteBrandItems.length > 0 && (
                    <div className="space-y-2 mb-4">
                      <p className="text-xs font-semibold uppercase tracking-widest text-red-400">
                        High Priority ({incompleteBrandItems.length + todos.filter(t => t.status === 'todo' && t.priority === 'high').length})
                      </p>
                      {incompleteBrandItems.map(item => (
                        <div
                          key={`brand-intel-${item.domain}`}
                          className="bg-amber-500/5 border border-amber-500/20 rounded-xl p-4 flex items-start gap-3"
                        >
                          <div className="w-5 h-5 rounded border-2 border-amber-500/50 shrink-0 mt-0.5 flex items-center justify-center bg-amber-500/10">
                            <svg className="w-3 h-3 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-white text-sm font-medium leading-snug">
                              Complete Brand Intelligence for {item.brandName}
                            </p>
                            <p className="text-dark-400 text-xs mt-1">
                              {item.completeness === 0
                                ? 'No brand profile set up yet. Brand Intelligence improves analysis and optimization accuracy.'
                                : `Brand profile is ${item.completeness}% complete. Complete it for better analysis and optimization results.`
                              }
                            </p>
                            <div className="flex items-center gap-3 mt-2">
                              <span className="text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider bg-red-500/20 text-red-400 border border-red-500/30">high</span>
                              <div className="flex items-center gap-1.5 flex-1 max-w-32">
                                <div className="h-1.5 bg-dark-800 rounded-full flex-1 overflow-hidden">
                                  <div className={`h-full rounded-full transition-all ${item.completeness >= 70 ? 'bg-emerald-500' : item.completeness >= 40 ? 'bg-amber-500' : 'bg-red-500'}`} style={{ width: `${item.completeness}%` }} />
                                </div>
                                <span className="text-dark-500 text-[10px]">{item.completeness}%</span>
                              </div>
                            </div>
                          </div>
                          {!readOnly && (
                            <button
                              onClick={() => {
                                const brand = brandList.find(b => b.domain === item.domain)
                                if (brand) { loadBrandProfile(item.domain); } else { startNewBrand(item.domain); }
                                setActiveTab('brand')
                              }}
                              className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all cursor-pointer whitespace-nowrap shrink-0"
                            >
                              {item.completeness === 0 ? 'Set Up' : 'Complete'}
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  )}

                  {todoSubTab === 'completed' && completeBrandItems.length > 0 && (
                    <div className="space-y-2 mb-4">
                      {completeBrandItems.map(item => (
                        <div
                          key={`brand-intel-done-${item.domain}`}
                          className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl p-4 flex items-start gap-3"
                        >
                          <div className="w-5 h-5 rounded border-2 border-emerald-500/50 shrink-0 mt-0.5 flex items-center justify-center bg-emerald-500/20">
                            <svg className="w-3 h-3 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-dark-500 text-sm font-medium leading-snug line-through">
                              Complete Brand Intelligence for {item.brandName}
                            </p>
                            <div className="flex items-center gap-2 mt-1">
                              <span className="text-emerald-500 text-xs">100% complete</span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )
            })()}

            {/* Todo list */}
            {todosLoading && todos.length === 0 ? (
              <div className="flex justify-center py-16">
                <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
              </div>
            ) : (() => {
              const filtered = todos.filter(t => t.status === todoSubTab && domainKey(t.domain) === domainKey(selectedDomain))
              if (filtered.length === 0) {
                // Don't show empty state for todo/completed if brand intel items filled the gap
                if (todoSubTab === 'backlogged') {
                  return (
                    <div className="text-center py-16 max-w-md mx-auto">
                      <svg className="w-12 h-12 mx-auto text-dark-600 mb-4" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m6 4.125l2.25 2.25m0 0l2.25 2.25M12 13.875l2.25-2.25M12 13.875l-2.25 2.25M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
                      <h3 className="text-white font-semibold text-lg mb-2">No Backlogged Items</h3>
                      <p className="text-dark-400 text-sm">Items that are deferred or archived will appear here.</p>
                    </div>
                  )
                }
                // For todo/completed, only show empty state if there are also no brand intel virtual items
                if (todoSubTab === 'completed') {
                  return (
                    <div className="text-center py-12 text-dark-500">
                      <p className="text-sm">No completed items yet. Complete a to-do to see it here.</p>
                    </div>
                  )
                }
                return null
              }

              const priorityOrder: Record<string, number> = { high: 0, medium: 1, low: 2 }

              /* ---- Sort by Priority: flat list ---- */
              if (todoSortMode === 'priority') {
                const sorted = [...filtered].sort((a, b) =>
                  (priorityOrder[a.priority] ?? 3) - (priorityOrder[b.priority] ?? 3)
                )

                // Group by priority level
                const byPriority: Record<string, TodoItem[]> = {}
                for (const t of sorted) {
                  const p = t.priority || 'low'
                  if (!byPriority[p]) byPriority[p] = []
                  byPriority[p].push(t)
                }

                const priorityLabels: Record<string, string> = { high: 'High Priority', medium: 'Medium Priority', low: 'Low Priority' }
                const priorityStyles: Record<string, string> = {
                  high: 'text-red-400',
                  medium: 'text-amber-400',
                  low: 'text-dark-400',
                }

                return (
                  <div className="space-y-6">
                    {['high', 'medium', 'low'].filter(p => byPriority[p]?.length).map(p => (
                      <div key={p}>
                        <p className={`text-xs font-semibold uppercase tracking-widest mb-3 ${priorityStyles[p]}`}>
                          {priorityLabels[p]} ({byPriority[p].length})
                        </p>
                        <div className="space-y-1.5">
                          {byPriority[p].map(todo => {
                            const isExpanded = expandedTodos.has(todo.id)
                            return (
                            <div
                              key={todo.id}
                              className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl overflow-hidden group hover:border-dark-700 transition-colors"
                            >
                              {/* Compact row — always visible */}
                              <div className="flex items-center gap-3 px-4 py-2.5">
                                {!readOnly && (
                                  <button
                                    onClick={(e) => { e.stopPropagation(); updateTodoStatus(todo.id, todo.status === 'completed' ? 'todo' : 'completed') }}
                                    className={`w-4.5 h-4.5 rounded border-2 shrink-0 flex items-center justify-center transition-all cursor-pointer ${
                                      todo.status === 'completed'
                                        ? 'bg-emerald-500/20 border-emerald-500/50'
                                        : 'border-dark-600 hover:border-primary-500'
                                    }`}
                                  >
                                    {todo.status === 'completed' && (
                                      <svg className="w-2.5 h-2.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                      </svg>
                                    )}
                                  </button>
                                )}
                                <button
                                  onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })}
                                  className="flex-1 min-w-0 text-left cursor-pointer"
                                >
                                  <p className={`text-sm leading-snug truncate ${
                                    todo.status === 'completed' ? 'text-dark-500 line-through' : 'text-white'
                                  }`}>
                                    {todo.summary || todo.action}
                                  </p>
                                </button>
                                <div className="flex items-center gap-1.5 shrink-0">
                                  <button
                                    onClick={() => {
                                      if (todo.source_type === 'video') { loadVideoAnalysis(todo.domain); setActiveTab('video') }
                                      else if (todo.source_type === 'reddit') { loadRedditAnalysis(todo.domain); setActiveTab('reddit') }
                                      else if (todo.source_type === 'search') { setActiveTab('search') }
                                      else if (todo.optimization_id) { loadOptimizationDetail(todo.optimization_id); setActiveTab('optimize') }
                                    }}
                                    className={`text-[9px] px-1.5 py-0.5 rounded font-medium cursor-pointer transition-colors ${
                                      todo.source_type === 'video' ? 'bg-purple-500/20 text-purple-300 border border-purple-500/30 hover:bg-purple-500/30'
                                      : todo.source_type === 'reddit' ? 'bg-orange-500/20 text-orange-300 border border-orange-500/30 hover:bg-orange-500/30'
                                      : todo.source_type === 'search' ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 hover:bg-cyan-500/30'
                                      : 'bg-primary-500/20 text-primary-300 border border-primary-500/30 hover:bg-primary-500/30'
                                    }`}
                                  >
                                    {todo.source_type === 'video' ? 'YouTube' : todo.source_type === 'reddit' ? 'Reddit' : todo.source_type === 'search' ? 'Search' : 'Optimize'}
                                  </button>
                                  {todo.tags?.includes('discovery') && (
                                    <span className="text-[9px] px-1.5 py-0.5 rounded font-medium bg-teal-500/20 text-teal-300 border border-teal-500/30">Discovery</span>
                                  )}
                                  <span className="text-dark-500 text-xs capitalize hidden sm:inline">{todo.dimension.replace(/_/g, ' ')}</span>
                                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                    {!readOnly && todo.status !== 'backlogged' && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'backlogged')} title="Backlog" className="p-1 text-dark-500 hover:text-amber-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'archived')} title="Archive" className="p-1 text-dark-500 hover:text-dark-300 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && (todo.status === 'backlogged' || todo.status === 'archived') && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'todo')} title="Move to To-Do" className="p-1 text-dark-500 hover:text-primary-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" /></svg>
                                      </button>
                                    )}
                                  </div>
                                  <svg className={`w-3.5 h-3.5 text-dark-600 transition-transform cursor-pointer ${isExpanded ? 'rotate-180' : ''}`} onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </div>
                              </div>

                              {/* Expanded details */}
                              {isExpanded && (
                                <div className="px-4 pb-3 pt-0 border-t border-dark-800/50 space-y-2">
                                  <p className="text-dark-300 text-xs leading-relaxed mt-2">{todo.action}</p>
                                  <p className="text-dark-400 text-xs">{todo.expected_impact}</p>
                                  <div className="flex items-center gap-2 flex-wrap text-[10px]">
                                    <span className="text-dark-500 truncate max-w-xs" title={todo.question}>{todo.question}</span>
                                    {todo.created_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Created {fmtDate(todo.created_at)}</span></>)}
                                    {todo.backlogged_at && (<><span className="text-dark-600">·</span><span className="text-amber-400/70">Backlogged {fmtDate(todo.backlogged_at)}</span></>)}
                                    {todo.completed_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Done {fmtDate(todo.completed_at)}</span></>)}
                                    {todo.archived_at && (<><span className="text-dark-600">·</span><span className="text-dark-400/70">Archived {fmtDate(todo.archived_at)}</span></>)}
                                  </div>
                                </div>
                              )}
                            </div>
                          )})}

                        </div>
                      </div>
                    ))}
                  </div>
                )
              }

              /* ---- Sort by Dimension: grouped by dimension ---- */
              if (todoSortMode === 'dimension') {
                const byDim: Record<string, TodoItem[]> = {}
                for (const t of filtered) {
                  const d = t.dimension || 'other'
                  if (!byDim[d]) byDim[d] = []
                  byDim[d].push(t)
                }
                // Sort within each group by priority
                for (const items of Object.values(byDim)) {
                  items.sort((a, b) => (priorityOrder[a.priority] ?? 3) - (priorityOrder[b.priority] ?? 3))
                }
                const dimLabels: Record<string, string> = {
                  content_authority: 'Content Authority',
                  source_authority: 'Source Authority',
                  structural_optimization: 'Structural Optimization',
                  knowledge_persistence: 'Knowledge Persistence',
                  other: 'Other',
                }
                const dimColors: Record<string, string> = {
                  content_authority: 'text-primary-400',
                  source_authority: 'text-accent-purple',
                  structural_optimization: 'text-accent-cyan',
                  knowledge_persistence: 'text-accent-emerald',
                  other: 'text-dark-400',
                }
                const dimOrder = ['content_authority', 'source_authority', 'structural_optimization', 'knowledge_persistence', 'other']
                return (
                  <div className="space-y-6">
                    {dimOrder.filter(d => byDim[d]?.length).map(d => (
                      <div key={d}>
                        <p className={`text-xs font-semibold uppercase tracking-widest mb-3 ${dimColors[d] || 'text-dark-400'}`}>
                          {dimLabels[d] || d.replace(/_/g, ' ')} ({byDim[d].length})
                        </p>
                        <div className="space-y-1.5">
                          {byDim[d].map(todo => {
                            const isExpanded = expandedTodos.has(todo.id)
                            return (
                            <div
                              key={todo.id}
                              className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl overflow-hidden group hover:border-dark-700 transition-colors"
                            >
                              <div className="flex items-center gap-3 px-4 py-2.5">
                                {!readOnly && (
                                  <button
                                    onClick={(e) => { e.stopPropagation(); updateTodoStatus(todo.id, todo.status === 'completed' ? 'todo' : 'completed') }}
                                    className={`w-4.5 h-4.5 rounded border-2 shrink-0 flex items-center justify-center transition-all cursor-pointer ${
                                      todo.status === 'completed'
                                        ? 'bg-emerald-500/20 border-emerald-500/50'
                                        : 'border-dark-600 hover:border-primary-500'
                                    }`}
                                  >
                                    {todo.status === 'completed' && (
                                      <svg className="w-2.5 h-2.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                      </svg>
                                    )}
                                  </button>
                                )}
                                <button
                                  onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })}
                                  className="flex-1 min-w-0 text-left cursor-pointer"
                                >
                                  <p className={`text-sm leading-snug truncate ${
                                    todo.status === 'completed' ? 'text-dark-500 line-through' : 'text-white'
                                  }`}>
                                    {todo.summary || todo.action}
                                  </p>
                                </button>
                                <div className="flex items-center gap-1.5 shrink-0">
                                  <button
                                    onClick={() => {
                                      if (todo.source_type === 'video') { loadVideoAnalysis(todo.domain); setActiveTab('video') }
                                      else if (todo.source_type === 'reddit') { loadRedditAnalysis(todo.domain); setActiveTab('reddit') }
                                      else if (todo.source_type === 'search') { setActiveTab('search') }
                                      else if (todo.optimization_id) { loadOptimizationDetail(todo.optimization_id); setActiveTab('optimize') }
                                    }}
                                    className={`text-[9px] px-1.5 py-0.5 rounded font-medium cursor-pointer transition-colors ${
                                      todo.source_type === 'video' ? 'bg-purple-500/20 text-purple-300 border border-purple-500/30 hover:bg-purple-500/30'
                                      : todo.source_type === 'reddit' ? 'bg-orange-500/20 text-orange-300 border border-orange-500/30 hover:bg-orange-500/30'
                                      : todo.source_type === 'search' ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 hover:bg-cyan-500/30'
                                      : 'bg-primary-500/20 text-primary-300 border border-primary-500/30 hover:bg-primary-500/30'
                                    }`}
                                  >
                                    {todo.source_type === 'video' ? 'YouTube' : todo.source_type === 'reddit' ? 'Reddit' : todo.source_type === 'search' ? 'Search' : 'Optimize'}
                                  </button>
                                  {todo.tags?.includes('discovery') && (
                                    <span className="text-[9px] px-1.5 py-0.5 rounded font-medium bg-teal-500/20 text-teal-300 border border-teal-500/30">Discovery</span>
                                  )}
                                  <span className={`text-[9px] px-1 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    todo.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    todo.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{todo.priority}</span>
                                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                    {!readOnly && todo.status !== 'backlogged' && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'backlogged')} title="Backlog" className="p-1 text-dark-500 hover:text-amber-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'archived')} title="Archive" className="p-1 text-dark-500 hover:text-dark-300 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && (todo.status === 'backlogged' || todo.status === 'archived') && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'todo')} title="Move to To-Do" className="p-1 text-dark-500 hover:text-primary-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" /></svg>
                                      </button>
                                    )}
                                  </div>
                                  <svg className={`w-3.5 h-3.5 text-dark-600 transition-transform cursor-pointer ${isExpanded ? 'rotate-180' : ''}`} onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </div>
                              </div>
                              {isExpanded && (
                                <div className="px-4 pb-3 pt-0 border-t border-dark-800/50 space-y-2">
                                  <p className="text-dark-300 text-xs leading-relaxed mt-2">{todo.action}</p>
                                  <p className="text-dark-400 text-xs">{todo.expected_impact}</p>
                                  <div className="flex items-center gap-2 flex-wrap text-[10px]">
                                    <span className="text-dark-500 truncate max-w-xs" title={todo.question}>{todo.question}</span>
                                    {todo.created_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Created {fmtDate(todo.created_at)}</span></>)}
                                  </div>
                                </div>
                              )}
                            </div>
                          )})}
                        </div>
                      </div>
                    ))}
                  </div>
                )
              }

              /* ---- Sort by Question: grouped by domain+question ---- */
              const groups: Record<string, TodoItem[]> = {}
              for (const t of filtered) {
                const key = `${t.domain}|||${t.question}`
                if (!groups[key]) groups[key] = []
                groups[key].push(t)
              }

              return (
                <div className="space-y-6">
                  {Object.entries(groups).map(([key, items]) => {
                    const [, question] = key.split('|||')
                    return (
                      <div key={key}>
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div>
                            <p className="text-white font-medium text-sm leading-snug">{question}</p>
                          </div>
                          {(items[0]?.optimization_id || items[0]?.video_analysis_id) && (
                            <button
                              onClick={() => {
                                if (items[0].source_type === 'video') {
                                  loadVideoAnalysis(items[0].domain)
                                  setActiveTab('video')
                                } else {
                                  loadOptimizationDetail(items[0].optimization_id)
                                  setActiveTab('optimize')
                                }
                              }}
                              className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer whitespace-nowrap shrink-0 flex items-center gap-1"
                            >
                              View report
                              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
                              </svg>
                            </button>
                          )}
                        </div>
                        <div className="space-y-1.5">
                          {items.map(todo => {
                            const isExpanded = expandedTodos.has(todo.id)
                            return (
                            <div
                              key={todo.id}
                              className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-xl overflow-hidden group hover:border-dark-700 transition-colors"
                            >
                              <div className="flex items-center gap-3 px-4 py-2.5">
                                {!readOnly && (
                                  <button
                                    onClick={(e) => { e.stopPropagation(); updateTodoStatus(todo.id, todo.status === 'completed' ? 'todo' : 'completed') }}
                                    className={`w-4.5 h-4.5 rounded border-2 shrink-0 flex items-center justify-center transition-all cursor-pointer ${
                                      todo.status === 'completed'
                                        ? 'bg-emerald-500/20 border-emerald-500/50'
                                        : 'border-dark-600 hover:border-primary-500'
                                    }`}
                                  >
                                    {todo.status === 'completed' && (
                                      <svg className="w-2.5 h-2.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                      </svg>
                                    )}
                                  </button>
                                )}
                                <button
                                  onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })}
                                  className="flex-1 min-w-0 text-left cursor-pointer"
                                >
                                  <p className={`text-sm leading-snug truncate ${
                                    todo.status === 'completed' ? 'text-dark-500 line-through' : 'text-white'
                                  }`}>
                                    {todo.summary || todo.action}
                                  </p>
                                </button>
                                <div className="flex items-center gap-1.5 shrink-0">
                                  <button
                                    onClick={() => {
                                      if (todo.source_type === 'video') { loadVideoAnalysis(todo.domain); setActiveTab('video') }
                                      else if (todo.source_type === 'reddit') { loadRedditAnalysis(todo.domain); setActiveTab('reddit') }
                                      else if (todo.source_type === 'search') { setActiveTab('search') }
                                      else if (todo.optimization_id) { loadOptimizationDetail(todo.optimization_id); setActiveTab('optimize') }
                                    }}
                                    className={`text-[9px] px-1.5 py-0.5 rounded font-medium cursor-pointer transition-colors ${
                                      todo.source_type === 'video' ? 'bg-purple-500/20 text-purple-300 border border-purple-500/30 hover:bg-purple-500/30'
                                      : todo.source_type === 'reddit' ? 'bg-orange-500/20 text-orange-300 border border-orange-500/30 hover:bg-orange-500/30'
                                      : todo.source_type === 'search' ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 hover:bg-cyan-500/30'
                                      : 'bg-primary-500/20 text-primary-300 border border-primary-500/30 hover:bg-primary-500/30'
                                    }`}
                                  >
                                    {todo.source_type === 'video' ? 'YouTube' : todo.source_type === 'reddit' ? 'Reddit' : todo.source_type === 'search' ? 'Search' : 'Optimize'}
                                  </button>
                                  {todo.tags?.includes('discovery') && (
                                    <span className="text-[9px] px-1.5 py-0.5 rounded font-medium bg-teal-500/20 text-teal-300 border border-teal-500/30">Discovery</span>
                                  )}
                                  <span className={`text-[9px] px-1 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    todo.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    todo.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{todo.priority}</span>
                                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                                    {!readOnly && todo.status !== 'backlogged' && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'backlogged')} title="Backlog" className="p-1 text-dark-500 hover:text-amber-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && todo.status !== 'archived' && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'archived')} title="Archive" className="p-1 text-dark-500 hover:text-dark-300 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
                                      </button>
                                    )}
                                    {!readOnly && (todo.status === 'backlogged' || todo.status === 'archived') && (
                                      <button onClick={() => updateTodoStatus(todo.id, 'todo')} title="Move to To-Do" className="p-1 text-dark-500 hover:text-primary-400 transition-colors cursor-pointer">
                                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" /></svg>
                                      </button>
                                    )}
                                  </div>
                                  <svg className={`w-3.5 h-3.5 text-dark-600 transition-transform cursor-pointer ${isExpanded ? 'rotate-180' : ''}`} onClick={() => setExpandedTodos(prev => { const next = new Set(prev); next.has(todo.id) ? next.delete(todo.id) : next.add(todo.id); return next })} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </div>
                              </div>
                              {isExpanded && (
                                <div className="px-4 pb-3 pt-0 border-t border-dark-800/50 space-y-2">
                                  <p className="text-dark-300 text-xs leading-relaxed mt-2">{todo.action}</p>
                                  <p className="text-dark-400 text-xs">{todo.expected_impact}</p>
                                  <div className="flex items-center gap-2 flex-wrap text-[10px]">
                                    <span className="text-dark-500 capitalize">{todo.dimension.replace(/_/g, ' ')}</span>
                                    {todo.created_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Created {fmtDate(todo.created_at)}</span></>)}
                                    {todo.backlogged_at && (<><span className="text-dark-600">·</span><span className="text-amber-400/70">Backlogged {fmtDate(todo.backlogged_at)}</span></>)}
                                    {todo.completed_at && (<><span className="text-dark-600">·</span><span className="text-dark-500">Done {fmtDate(todo.completed_at)}</span></>)}
                                    {todo.archived_at && (<><span className="text-dark-600">·</span><span className="text-dark-400/70">Archived {fmtDate(todo.archived_at)}</span></>)}
                                  </div>
                                </div>
                              )}
                            </div>
                          )})}

                        </div>
                      </div>
                    )
                  })}
                </div>
              )
            })()}
          </div>
        )}

        {/* ===== STATUS TAB ===== */}
        {activeTab === 'status' && (
          <div className="max-w-2xl mx-auto animate-fade-in space-y-6">
            {/* API Key Status Banner */}
            {saasEnabled && user && !configLoaded && (
              <div className="flex items-center gap-3 p-4 bg-dark-800/50 border border-dark-800 rounded-xl animate-pulse">
                <span className="w-5 h-5 rounded bg-dark-700 shrink-0" />
                <div className="flex-1 space-y-1.5">
                  <span className="block w-48 h-3.5 rounded bg-dark-700" />
                  <span className="block w-64 h-2.5 rounded bg-dark-700" />
                </div>
                <span className="w-20 h-7 rounded-lg bg-dark-700" />
              </div>
            )}
            {saasEnabled && user && configLoaded && apiKeyStatus === 'unconfigured' && (
              <div className="flex items-center gap-3 p-4 bg-amber-500/10 border border-amber-500/20 rounded-xl">
                <svg className="w-5 h-5 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                <div className="flex-1">
                  <p className="text-sm text-amber-300 font-medium">Configure your API key to start analyzing</p>
                  <p className="text-xs text-dark-400 mt-0.5">You need an API key to use AI analysis features.</p>
                </div>
                <a href="/last/settings" className="px-3 py-1.5 text-sm bg-amber-500/20 text-amber-300 rounded-lg hover:bg-amber-500/30 transition-colors whitespace-nowrap">Go to Settings</a>
              </div>
            )}
            {saasEnabled && user && apiKeyStatus === 'no_credits' && (
              <div className="flex items-center gap-3 p-4 bg-amber-500/10 border border-amber-500/20 rounded-xl">
                <span className="text-amber-400 text-lg shrink-0">$</span>
                <div className="flex-1">
                  <p className="text-sm text-amber-300 font-medium">Your API key needs credits</p>
                  <p className="text-xs text-dark-400 mt-0.5">Add credits to your AI provider account to continue analyzing.</p>
                </div>
                <a href="/last/settings" className="px-3 py-1.5 text-sm bg-amber-500/20 text-amber-300 rounded-lg hover:bg-amber-500/30 transition-colors whitespace-nowrap">Settings</a>
              </div>
            )}
            {saasEnabled && user && apiKeyStatus === 'invalid' && (
              <div className="flex items-center gap-3 p-4 bg-red-500/10 border border-red-500/20 rounded-xl">
                <svg className="w-5 h-5 text-red-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                <div className="flex-1">
                  <p className="text-sm text-red-300 font-medium">Your API key is invalid</p>
                  <p className="text-xs text-dark-400 mt-0.5">Please update your API key in Settings.</p>
                </div>
                <a href="/last/settings" className="px-3 py-1.5 text-sm bg-red-500/20 text-red-300 rounded-lg hover:bg-red-500/30 transition-colors whitespace-nowrap">Settings</a>
              </div>
            )}
            {saasEnabled && user && apiKeyStatus === 'active' && (
              <div className="flex items-center gap-2 p-3 bg-emerald-500/5 border border-emerald-500/20 rounded-xl">
                <svg className="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                <p className="text-sm text-emerald-400">Your API key is active</p>
              </div>
            )}

            {/* Current Status Card */}
            <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                  API Provider Status
                </h3>
                <button
                  onClick={checkHealth}
                  disabled={healthChecking}
                  className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all disabled:opacity-50 cursor-pointer"
                >
                  {healthChecking ? 'Checking...' : 'Check Now'}
                </button>
              </div>

              {healthHistory.length === 0 ? (
                <div className="flex items-center justify-center py-8">
                  {healthChecking ? (
                    <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                  ) : (
                    <p className="text-dark-500 text-sm">No checks performed yet</p>
                  )}
                </div>
              ) : (
                <div className="space-y-4">
                  {/* Per-model status rows */}
                  <div className="space-y-3">
                    {healthHistory[0].models.map((m) => (
                      <div key={m.model} className={`flex items-center gap-4 p-3 rounded-xl border ${
                        m.status === 'available'
                          ? 'bg-emerald-500/5 border-emerald-500/20'
                          : m.status === 'overloaded'
                          ? 'bg-amber-500/5 border-amber-500/20'
                          : 'bg-red-500/5 border-red-500/20'
                      }`}>
                        <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${
                          m.status === 'available'
                            ? 'bg-emerald-500/20'
                            : m.status === 'overloaded'
                            ? 'bg-amber-500/20'
                            : 'bg-red-500/20'
                        }`}>
                          {m.status === 'available' ? (
                            <svg className="w-5 h-5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                            </svg>
                          ) : m.status === 'overloaded' ? (
                            <svg className="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                            </svg>
                          ) : (
                            <svg className="w-5 h-5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          )}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="text-white font-medium text-sm">{m.name}</span>
                            <span className={`text-xs font-medium ${
                              m.status === 'available' ? 'text-emerald-400' :
                              m.status === 'overloaded' ? 'text-amber-400' :
                              'text-red-400'
                            }`}>
                              {m.status === 'available' ? 'Available' :
                               m.status === 'overloaded' ? 'Overloaded' :
                               'Error'}
                            </span>
                          </div>
                          <p className="text-dark-500 text-xs truncate">
                            {m.model}
                            {m.latency_ms !== undefined && ` · ${m.latency_ms}ms`}
                          </p>
                          {m.error && m.status !== 'available' && (
                            <p className="text-dark-500 text-xs mt-1 break-all line-clamp-3">
                              {(() => {
                                try {
                                  const parsed = JSON.parse(m.error)
                                  if (parsed.error?.message) return parsed.error.message
                                  if (parsed.message) return parsed.message
                                  return m.error
                                } catch {
                                  return m.error
                                }
                              })()}
                            </p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>

                  {/* Partial outage indicator */}
                  {healthHistory[0].models.some(m => m.status !== 'available') &&
                   healthHistory[0].models.some(m => m.status === 'available') && (
                    <div className="bg-amber-500/10 border border-amber-500/20 rounded-lg p-3">
                      <p className="text-amber-300 text-sm">
                        {healthHistory[0].models.filter(m => m.status !== 'available').map(m => m.name).join(', ')} {healthHistory[0].models.filter(m => m.status !== 'available').length > 1 ? 'have' : 'has'} issues
                      </p>
                    </div>
                  )}

                  <p className="text-dark-500 text-xs">
                    Last checked: {new Date(healthHistory[0].checked_at).toLocaleTimeString(undefined, {
                      hour: '2-digit', minute: '2-digit', second: '2-digit'
                    })}
                    {' · '}Auto-checks every 5 minutes
                  </p>
                </div>
              )}
            </div>

            {/* Availability Timeline */}
            <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest">
                  Availability Timeline (24h)
                </h3>
                {healthTimelineLoading && (
                  <div className="w-4 h-4 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                )}
              </div>
              {healthTimeline.length === 0 ? (
                <p className="text-dark-500 text-sm text-center py-6">No historical data yet. Checks are recorded every 5 minutes.</p>
              ) : (() => {
                // Extract model names from most recent record (last element, sorted ascending)
                const latest = healthTimeline[healthTimeline.length - 1]
                const modelNames = latest?.models.map(m => ({ model: m.model, name: m.name })) || []
                // Filter to records matching current format (skip old claude-specific records)
                const modelSet = new Set(modelNames.map(m => m.model))
                const relevantTimeline = healthTimeline.filter(r => r.models.some(m => modelSet.has(m.model)))
                return (
                  <div className="space-y-4">
                    {modelNames.map(({ model, name }) => {
                      const statuses = relevantTimeline.map(r => {
                        const m = r.models.find(mm => mm.model === model)
                        return { status: m?.status || 'error', time: r.checked_at, latency: m?.latency_ms }
                      })
                      const available = statuses.filter(s => s.status === 'available').length
                      const uptime = statuses.length > 0 ? Math.round((available / statuses.length) * 100) : 0
                      return (
                        <div key={model}>
                          <div className="flex items-center justify-between mb-2">
                            <div className="flex items-center gap-2">
                              <span className="text-white text-sm font-medium">{name}</span>
                            </div>
                            <span className={`text-xs font-medium ${uptime >= 95 ? 'text-emerald-400' : uptime >= 80 ? 'text-amber-400' : 'text-red-400'}`}>
                              {uptime}% uptime
                            </span>
                          </div>
                          {/* Visual bar of status dots */}
                          <div className="flex gap-px items-center" title={`${statuses.length} checks`}>
                            {statuses.length <= 120 ? (
                              statuses.map((s, i) => (
                                <div
                                  key={i}
                                  className={`flex-1 h-6 rounded-sm min-w-[2px] ${
                                    s.status === 'available' ? 'bg-emerald-500/60' :
                                    s.status === 'overloaded' ? 'bg-amber-500/60' :
                                    'bg-red-500/60'
                                  }`}
                                  title={`${new Date(s.time).toLocaleTimeString()} — ${s.status}${s.latency ? ` (${s.latency}ms)` : ''}`}
                                />
                              ))
                            ) : (
                              // Downsample to 120 bins for display
                              (() => {
                                const binSize = Math.ceil(statuses.length / 120)
                                const bins: { status: string; time: string }[] = []
                                for (let i = 0; i < statuses.length; i += binSize) {
                                  const chunk = statuses.slice(i, i + binSize)
                                  const worst = chunk.find(c => c.status === 'error') || chunk.find(c => c.status === 'overloaded') || chunk[0]
                                  bins.push({ status: worst.status, time: chunk[0].time })
                                }
                                return bins.map((s, i) => (
                                  <div
                                    key={i}
                                    className={`flex-1 h-6 rounded-sm min-w-[2px] ${
                                      s.status === 'available' ? 'bg-emerald-500/60' :
                                      s.status === 'overloaded' ? 'bg-amber-500/60' :
                                      'bg-red-500/60'
                                    }`}
                                    title={`${new Date(s.time).toLocaleTimeString()} — ${s.status}`}
                                  />
                                ))
                              })()
                            )}
                          </div>
                          <div className="flex justify-between mt-1">
                            <span className="text-dark-600 text-[10px]">
                              {new Date(statuses[0]?.time).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                            </span>
                            <span className="text-dark-600 text-[10px]">
                              {new Date(statuses[statuses.length - 1]?.time).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                            </span>
                          </div>
                        </div>
                      )
                    })}
                    {/* Legend */}
                    <div className="flex items-center gap-4 pt-2">
                      <div className="flex items-center gap-1.5"><div className="w-3 h-3 rounded-sm bg-emerald-500/60" /><span className="text-dark-500 text-[10px]">Available</span></div>
                      <div className="flex items-center gap-1.5"><div className="w-3 h-3 rounded-sm bg-amber-500/60" /><span className="text-dark-500 text-[10px]">Overloaded</span></div>
                      <div className="flex items-center gap-1.5"><div className="w-3 h-3 rounded-sm bg-red-500/60" /><span className="text-dark-500 text-[10px]">Error</span></div>
                      <span className="text-dark-600 text-[10px] ml-auto">{relevantTimeline.length} checks in last 24h</span>
                    </div>
                  </div>
                )
              })()}
            </div>
          </div>
        )}

        {/* ===== BRAND TAB ===== */}
        {activeTab === 'brand' && (
          <div className="max-w-3xl mx-auto animate-fade-in">
            {!brandEditing ? (
              /* Loading / auto-loading brand for selectedDomain */
              <div className="flex justify-center py-16">
                <div className="w-6 h-6 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
              </div>
            ) : (
              /* Brand Edit View */
              <div>
                <div className="flex items-center justify-between mb-6">
                  {!readOnly ? (
                    <button
                      onClick={() => guardBrandNav(() => { setBrandEditing(false); fetchBrandList() })}
                      className="text-dark-400 hover:text-white text-sm flex items-center gap-1 cursor-pointer"
                    >
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
                      </svg>
                      Back
                    </button>
                  ) : (
                    <h3 className="text-sm font-semibold text-dark-400">Brand Intelligence</h3>
                  )}
                  {brandEditing && !readOnly && (
                    <div className="flex items-center gap-3">
                      {/* Completeness */}
                      <div className="flex items-center gap-2 text-xs">
                        <div className="w-24 h-1.5 bg-dark-800 rounded-full overflow-hidden">
                          <div
                            className={`h-full rounded-full transition-all ${
                              brandCompleteness >= 80 ? 'bg-emerald-500' :
                              brandCompleteness >= 50 ? 'bg-amber-500' : 'bg-orange-500'
                            }`}
                            style={{ width: `${brandCompleteness}%` }}
                          />
                        </div>
                        <span className="text-dark-500">{brandCompleteness}%</span>
                      </div>
                      {brandCompleteness < 50 && (
                        <button
                          onClick={quickSetupBrand}
                          disabled={quickSetupRunning}
                          className="text-xs px-3 py-1.5 bg-accent-purple/10 border border-accent-purple/20 text-purple-300 rounded-lg hover:bg-accent-purple/20 hover:text-purple-200 transition-all cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
                        >
                          {quickSetupRunning ? (
                            <>
                              <div className="w-3 h-3 border-2 border-purple-400/30 border-t-purple-400 rounded-full animate-spin" />
                              <span className="max-w-[160px] truncate">{quickSetupStep}</span>
                            </>
                          ) : (
                            <>
                              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" /></svg>
                              Quick Setup
                            </>
                          )}
                        </button>
                      )}
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      <button
                        onClick={saveBrandProfile}
                        disabled={brandSaving || !brandDomain.trim()}
                        className="px-4 py-1.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 cursor-pointer"
                      >
                        {brandSaving ? 'Saving...' : 'Save'}
                      </button>
                    </div>
                  )}
                </div>


                <div className={readOnly ? 'pointer-events-none' : ''}>
                {/* Section 1: Core Identity (always open) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6 mb-4">
                  <h4 className="text-sm font-semibold text-white mb-4">Core Identity</h4>
                  <div className="space-y-4">
                    <div>
                      <label className="block text-xs text-dark-400 mb-1">Brand Name</label>
                      <input
                        type="text"
                        value={brandForm.brand_name}
                        onChange={e => setBrandForm(p => ({ ...p, brand_name: e.target.value }))}
                        placeholder="Your Company Name"
                        className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                      />
                    </div>
                    <div>
                      <div className="flex items-center justify-between mb-1">
                        <label className="text-xs text-dark-400">Description</label>
                        <button
                          onClick={generateDescription}
                          disabled={generatingDesc || !brandDomain.trim()}
                          className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                        >
                          {generatingDesc ? 'Generating...' : 'Generate from site'}
                        </button>
                      </div>
                      {generatingDesc && generateMessages.length > 0 && (
                        <div className="mb-2 space-y-1">
                          {generateMessages.slice(-3).map((msg, i) => (
                            <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                              <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
                              {msg}
                            </div>
                          ))}
                        </div>
                      )}
                      <textarea
                        value={brandForm.description}
                        onChange={e => setBrandForm(p => ({ ...p, description: e.target.value }))}
                        placeholder="2-3 sentences about what this company does..."
                        rows={3}
                        className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm resize-none"
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Categories <span className="text-dark-600">(comma-separated)</span></label>
                        <input
                          type="text"
                          value={brandForm.categories}
                          onChange={e => setBrandForm(p => ({ ...p, categories: e.target.value }))}
                          placeholder="SaaS, project management"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Products / Features</label>
                        <input
                          type="text"
                          value={brandForm.products}
                          onChange={e => setBrandForm(p => ({ ...p, products: e.target.value }))}
                          placeholder="Gantt charts, API integrations"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                    </div>
                  </div>
                </div>

                {/* Section 2: Target Audience (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, audience: !p.audience }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">Target Audience</h4>
                      {(brandForm.primary_audience && brandForm.key_use_cases
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.audience ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs info</span> : null
                      )}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.audience ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.audience && (
                    <div className="px-6 pb-6 space-y-4">
                      <div className="flex items-center justify-end">
                        <button
                          onClick={predictAudience}
                          disabled={predictingAudience || !brandDomain.trim()}
                          className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer disabled:opacity-50"
                        >
                          {predictingAudience ? 'Predicting...' : 'Predict based on Website'}
                        </button>
                      </div>
                      {predictingAudience && predictAudienceMessages.length > 0 && (
                        <div className="space-y-1">
                          {predictAudienceMessages.slice(-3).map((msg, i) => (
                            <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                              {i === predictAudienceMessages.slice(-3).length - 1
                                ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse shrink-0" />
                                : <svg className="w-3 h-3 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                              }
                              {msg}
                            </div>
                          ))}
                        </div>
                      )}
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Primary Audience</label>
                        <textarea
                          value={brandForm.primary_audience}
                          onChange={e => setBrandForm(p => ({ ...p, primary_audience: e.target.value }))}
                          placeholder="Small business owners, engineering managers..."
                          rows={2}
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm resize-none"
                        />
                      </div>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Key Use Cases <span className="text-dark-600">(comma-separated)</span></label>
                        <input
                          type="text"
                          value={brandForm.key_use_cases}
                          onChange={e => setBrandForm(p => ({ ...p, key_use_cases: e.target.value }))}
                          placeholder="managing remote teams, sprint planning"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                    </div>
                  )}
                </div>

                {/* Section 3: Competitive Landscape (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, competitors: !p.competitors }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">
                        Competitive Landscape
                        {brandCompetitors.length > 0 && <span className="text-dark-500 font-normal ml-2">({brandCompetitors.length})</span>}
                      </h4>
                      {(brandCompetitors.length >= 3
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.competitors ? (brandCompetitors.length > 0
                            ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Add more ({brandCompetitors.length}/3)</span>
                            : <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs info</span>) : null
                      )}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.competitors ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.competitors && (
                    <div className="px-6 pb-6">
                      {/* Existing competitors list */}
                      {brandCompetitors.length > 0 && (
                        <div className="space-y-2 mb-4">
                          {brandCompetitors.map((comp, i) => (
                            <div key={i} className="flex items-center gap-2 bg-dark-800/50 border border-dark-700 rounded-lg p-3">
                              <div className="flex-1 grid grid-cols-4 gap-2">
                                <input
                                  type="text"
                                  value={comp.name}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, name: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  placeholder="Name"
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                />
                                <input
                                  type="text"
                                  value={comp.url}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, url: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  placeholder="URL"
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                />
                                <select
                                  value={comp.relationship}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, relationship: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                >
                                  <option value="direct">Direct</option>
                                  <option value="indirect">Indirect</option>
                                  <option value="aspirational">Aspirational</option>
                                  <option value="adjacent">Adjacent</option>
                                </select>
                                <input
                                  type="text"
                                  value={comp.notes}
                                  onChange={e => {
                                    const updated = [...brandCompetitors]
                                    updated[i] = { ...comp, notes: e.target.value }
                                    setBrandCompetitors(updated)
                                  }}
                                  placeholder="Notes"
                                  className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                />
                              </div>
                              <button
                                onClick={() => setBrandCompetitors(prev => prev.filter((_, j) => j !== i))}
                                className="text-dark-600 hover:text-red-400 cursor-pointer p-1"
                              >
                                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                              </button>
                            </div>
                          ))}
                        </div>
                      )}
                      <div className="flex gap-2">
                        <button
                          onClick={() => setBrandCompetitors(prev => [...prev, { name: '', url: '', relationship: 'direct', notes: '' }])}
                          className="text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg px-3 py-1.5 cursor-pointer"
                        >
                          + Add Manually
                        </button>
                        <button
                          onClick={discoverCompetitors}
                          disabled={discovering || !brandDomain.trim()}
                          className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                        >
                          {discovering ? 'Discovering...' : 'Discover Competitors'}
                        </button>
                      </div>

                      {/* Discovery progress */}
                      {discovering && discoverMessages.length > 0 && (
                        <div className="mt-4 space-y-1">
                          {discoverMessages.slice(-5).map((msg, i) => (
                            <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                              {i === discoverMessages.slice(-5).length - 1
                                ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
                                : <svg className="w-3 h-3 text-accent-emerald" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                              }
                              {msg}
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Discovery results */}
                      {discoveredCompetitors.length > 0 && (
                        <div className="mt-4">
                          <div className="flex items-center justify-between mb-2">
                            <span className="text-xs font-medium text-dark-400">Discovered ({discoveredCompetitors.length})</span>
                            <div className="flex gap-2">
                              <button
                                onClick={() => setDiscoverSelected(new Set(discoveredCompetitors.map((_, i) => i)))}
                                className="text-[10px] text-dark-500 hover:text-white cursor-pointer"
                              >
                                Select All
                              </button>
                              <button
                                onClick={addSelectedCompetitors}
                                disabled={discoverSelected.size === 0}
                                className={`text-xs px-3 py-1 rounded-lg cursor-pointer disabled:opacity-50 transition-all font-medium ${discoverSelected.size > 0 ? 'bg-primary-600 text-white hover:bg-primary-500 animate-pulse' : 'bg-dark-700 text-dark-400'}`}
                              >
                                Add Selected ({discoverSelected.size})
                              </button>
                            </div>
                          </div>
                          <div className="space-y-1">
                            {discoveredCompetitors.map((dc, i) => (
                              <label key={i} className="flex items-center gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-2.5 cursor-pointer hover:border-dark-600">
                                <input
                                  type="checkbox"
                                  checked={discoverSelected.has(i)}
                                  onChange={() => {
                                    setDiscoverSelected(prev => {
                                      const next = new Set(prev)
                                      if (next.has(i)) next.delete(i)
                                      else next.add(i)
                                      return next
                                    })
                                  }}
                                  className="accent-primary-500"
                                />
                                <div className="flex-1 min-w-0">
                                  <div className="flex items-center gap-2">
                                    <span className="text-white text-xs font-medium">{dc.name}</span>
                                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                                      dc.relationship === 'direct' ? 'bg-primary-500/20 text-primary-300' :
                                      dc.relationship === 'indirect' ? 'bg-amber-500/20 text-amber-300' :
                                      'bg-dark-700 text-dark-400'
                                    }`}>{dc.relationship}</span>
                                    <span className="text-[10px] text-dark-600">{dc.source}</span>
                                  </div>
                                  {dc.url && <span className="text-dark-500 text-[10px] truncate block">{dc.url}</span>}
                                </div>
                              </label>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Section 4: Key Queries & Brand Voice (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, queries: !p.queries }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">
                        Key Queries & Brand Voice
                        {(brandQueries.length > 0 || brandMessages.length > 0) && (
                          <span className="text-dark-500 font-normal ml-2">({brandQueries.length} queries, {brandMessages.length} claims)</span>
                        )}
                      </h4>
                      {(() => {
                        const hasQueries = brandQueries.length >= 5
                        const hasMessages = brandMessages.length > 0
                        const hasDiffs = !!brandForm.differentiators
                        const allDone = hasQueries && hasMessages && hasDiffs
                        const someDone = brandQueries.length > 0 || hasMessages || hasDiffs
                        return allDone
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.queries ? (someDone
                            ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Incomplete</span>
                            : <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs info</span>) : null
                      })()}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.queries ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.queries && (
                    <div className="px-6 pb-6 space-y-6">
                      {/* Target Queries */}
                      <div>
                        <label className="block text-xs font-medium text-dark-400 mb-1">Target Queries</label>
                        <p className="text-[11px] text-dark-500 mb-2">The most important questions you aspire to answer for users — whether or not they're answered on your site already.</p>
                        {brandQueries.length > 0 && (
                          <div className="space-y-1.5 mb-3">
                            {brandQueries.map((q, i) => (
                              <div key={i} className="flex items-center gap-2 bg-dark-800/50 border border-dark-700 rounded-lg p-2.5">
                                <div className="flex-1 grid grid-cols-6 gap-2">
                                  <input
                                    type="text"
                                    value={q.query}
                                    onChange={e => {
                                      const updated = [...brandQueries]
                                      updated[i] = { ...q, query: e.target.value }
                                      setBrandQueries(updated)
                                    }}
                                    placeholder="Query text"
                                    className="col-span-3 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                  />
                                  <select
                                    value={q.priority}
                                    onChange={e => {
                                      const updated = [...brandQueries]
                                      updated[i] = { ...q, priority: e.target.value }
                                      setBrandQueries(updated)
                                    }}
                                    className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                  >
                                    <option value="high">High</option>
                                    <option value="medium">Medium</option>
                                    <option value="low">Low</option>
                                  </select>
                                  <select
                                    value={q.type}
                                    onChange={e => {
                                      const updated = [...brandQueries]
                                      updated[i] = { ...q, type: e.target.value }
                                      setBrandQueries(updated)
                                    }}
                                    className="col-span-2 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                  >
                                    <option value="brand">Brand</option>
                                    <option value="category">Category</option>
                                    <option value="comparison">Comparison</option>
                                    <option value="problem">Problem</option>
                                  </select>
                                </div>
                                <button
                                  onClick={() => setBrandQueries(prev => prev.filter((_, j) => j !== i))}
                                  className="text-dark-600 hover:text-red-400 cursor-pointer p-1"
                                >
                                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                  </svg>
                                </button>
                              </div>
                            ))}
                          </div>
                        )}
                        <div className="flex gap-2">
                          <button
                            onClick={() => setBrandQueries(prev => [...prev, { query: '', priority: 'medium', type: 'category' }])}
                            className="text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg px-3 py-1.5 cursor-pointer"
                          >
                            + Add Query
                          </button>
                          <button
                            onClick={suggestQueries}
                            disabled={suggestingQueries || !brandDomain.trim()}
                            className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                          >
                            {suggestingQueries ? 'Suggesting...' : 'Suggest Queries'}
                          </button>
                        </div>

                        {/* Suggest progress */}
                        {suggestMessages.length > 0 && (
                          <div className="mt-3 space-y-1">
                            {suggestMessages.slice(-3).map((msg, i) => (
                              <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                                {suggestingQueries
                                  ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse" />
                                  : <div className="w-1.5 h-1.5 rounded-full bg-dark-600" />
                                }
                                <span className={msg.startsWith('Error:') ? 'text-red-400' : ''}>{msg}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Suggested queries */}
                        {suggestedQueries.length > 0 && (
                          <div className="mt-4">
                            <div className="flex items-center justify-between mb-2">
                              <span className="text-xs font-medium text-dark-400">Suggested ({suggestedQueries.length})</span>
                              <div className="flex gap-2">
                                <button
                                  onClick={() => setSuggestSelected(new Set(suggestedQueries.map((_, i) => i)))}
                                  className="text-[10px] text-dark-500 hover:text-white cursor-pointer"
                                >
                                  Select All
                                </button>
                                <button
                                  onClick={addSelectedQueries}
                                  disabled={suggestSelected.size === 0}
                                  className={`text-xs px-3 py-1 rounded-lg cursor-pointer disabled:opacity-50 transition-all font-medium ${suggestSelected.size > 0 ? 'bg-primary-600 text-white hover:bg-primary-500 animate-pulse' : 'bg-dark-700 text-dark-400'}`}
                                >
                                  Add Selected ({suggestSelected.size})
                                </button>
                              </div>
                            </div>
                            <div className="space-y-1 max-h-64 overflow-y-auto">
                              {suggestedQueries.map((sq, i) => (
                                <label key={i} className="flex items-center gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-2 cursor-pointer hover:border-dark-600">
                                  <input
                                    type="checkbox"
                                    checked={suggestSelected.has(i)}
                                    onChange={() => {
                                      setSuggestSelected(prev => {
                                        const next = new Set(prev)
                                        if (next.has(i)) next.delete(i)
                                        else next.add(i)
                                        return next
                                      })
                                    }}
                                    className="accent-primary-500"
                                  />
                                  <span className="flex-1 text-white text-xs">{sq.query}</span>
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                                    sq.priority === 'high' ? 'bg-red-500/20 text-red-300' :
                                    sq.priority === 'medium' ? 'bg-amber-500/20 text-amber-300' :
                                    'bg-dark-700 text-dark-400'
                                  }`}>{sq.priority}</span>
                                  <span className="text-[10px] text-dark-500">{sq.type}</span>
                                </label>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>

                      {/* Key Messages */}
                      <div>
                        <label className="block text-xs font-medium text-dark-400 mb-1">Key Brand Claims</label>
                        <p className="text-[11px] text-dark-500 mb-2">Discovers claims from your site content. Add any missing claims manually for a complete picture of your brand aspirations.</p>
                        {brandMessages.length > 0 && (
                          <div className="space-y-1.5 mb-3">
                            {brandMessages.map((m, i) => (
                              <div key={i} className="flex items-center gap-2 bg-dark-800/50 border border-dark-700 rounded-lg p-2.5">
                                <div className="flex-1 grid grid-cols-6 gap-2">
                                  <input
                                    type="text"
                                    value={m.claim}
                                    onChange={e => {
                                      const updated = [...brandMessages]
                                      updated[i] = { ...m, claim: e.target.value }
                                      setBrandMessages(updated)
                                    }}
                                    placeholder="Brand claim"
                                    className="col-span-3 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                  />
                                  <input
                                    type="text"
                                    value={m.evidence_url}
                                    onChange={e => {
                                      const updated = [...brandMessages]
                                      updated[i] = { ...m, evidence_url: e.target.value }
                                      setBrandMessages(updated)
                                    }}
                                    placeholder="Evidence URL"
                                    className="col-span-2 px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-xs"
                                  />
                                  <select
                                    value={m.priority}
                                    onChange={e => {
                                      const updated = [...brandMessages]
                                      updated[i] = { ...m, priority: e.target.value }
                                      setBrandMessages(updated)
                                    }}
                                    className="px-2 py-1.5 bg-dark-900 border border-dark-700 rounded text-white text-xs cursor-pointer"
                                  >
                                    <option value="high">High</option>
                                    <option value="medium">Medium</option>
                                    <option value="low">Low</option>
                                  </select>
                                </div>
                                <button
                                  onClick={() => setBrandMessages(prev => prev.filter((_, j) => j !== i))}
                                  className="text-dark-600 hover:text-red-400 cursor-pointer p-1"
                                >
                                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                                  </svg>
                                </button>
                              </div>
                            ))}
                          </div>
                        )}
                        <div className="flex gap-2">
                          <button
                            onClick={() => setBrandMessages(prev => [...prev, { claim: '', evidence_url: '', priority: 'medium' }])}
                            className="text-xs text-dark-400 hover:text-white border border-dark-700 rounded-lg px-3 py-1.5 cursor-pointer"
                          >
                            + Add Claim
                          </button>
                          <button
                            onClick={suggestClaims}
                            disabled={suggestingClaims || !brandDomain.trim()}
                            className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                          >
                            {suggestingClaims ? 'Discovering...' : 'Suggest Claims'}
                          </button>
                        </div>

                        {/* Suggest Claims progress */}
                        {suggestClaimMessages.length > 0 && (
                          <div className="mt-3 space-y-1">
                            {suggestClaimMessages.slice(-5).map((msg, i) => (
                              <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                                {suggestingClaims && i === suggestClaimMessages.slice(-5).length - 1
                                  ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse shrink-0" />
                                  : msg.startsWith('Error:')
                                    ? <div className="w-1.5 h-1.5 rounded-full bg-red-400 shrink-0" />
                                    : <svg className="w-3 h-3 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                                }
                                <span className={msg.startsWith('Error:') ? 'text-red-400' : ''}>{msg}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Suggested Claims review */}
                        {suggestedClaims.length > 0 && (
                          <div className="mt-3 border border-primary-500/20 rounded-xl p-4 bg-primary-500/5">
                            <div className="flex items-center justify-between mb-3">
                              <span className="text-xs font-semibold text-white">Discovered ({suggestedClaims.length})</span>
                              <div className="flex gap-2">
                                <button onClick={() => setSuggestClaimSelected(new Set(suggestedClaims.map((_, i) => i)))} className="text-[10px] text-dark-400 hover:text-white cursor-pointer">Select All</button>
                                <button
                                  onClick={addSelectedClaims}
                                  disabled={suggestClaimSelected.size === 0}
                                  className={`text-xs px-3 py-1 rounded-lg cursor-pointer disabled:opacity-50 transition-all font-medium ${suggestClaimSelected.size > 0 ? 'bg-primary-600 text-white hover:bg-primary-500 animate-pulse' : 'bg-dark-700 text-dark-400'}`}
                                >
                                  Add Selected ({suggestClaimSelected.size})
                                </button>
                              </div>
                            </div>
                            <div className="space-y-1.5 max-h-64 overflow-y-auto">
                              {suggestedClaims.map((sc, i) => (
                                <label key={i} className="flex items-start gap-3 p-2 rounded-lg hover:bg-dark-800/50 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={suggestClaimSelected.has(i)}
                                    onChange={() => {
                                      setSuggestClaimSelected(prev => {
                                        const next = new Set(prev)
                                        if (next.has(i)) next.delete(i); else next.add(i)
                                        return next
                                      })
                                    }}
                                    className="accent-primary-500 mt-0.5"
                                  />
                                  <div className="flex-1 min-w-0">
                                    <span className="text-white text-xs">{sc.claim}</span>
                                    <div className="flex items-center gap-2 mt-0.5">
                                      <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase ${
                                        sc.priority === 'high' ? 'bg-red-500/20 text-red-400' : sc.priority === 'medium' ? 'bg-amber-500/20 text-amber-400' : 'bg-dark-700/50 text-dark-400'
                                      }`}>{sc.priority}</span>
                                      {sc.evidence_url && <span className="text-dark-500 text-[10px] truncate">{sc.evidence_url}</span>}
                                    </div>
                                  </div>
                                </label>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>

                      {/* Differentiators */}
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Differentiators <span className="text-dark-600">(comma-separated)</span></label>
                        <input
                          type="text"
                          value={brandForm.differentiators}
                          onChange={e => setBrandForm(p => ({ ...p, differentiators: e.target.value }))}
                          placeholder="AI-powered, no per-seat pricing, open API"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                        <div className="flex gap-2 mt-2">
                          <button
                            onClick={predictDifferentiators}
                            disabled={predictingDiffs || !brandDomain.trim()}
                            className="text-xs text-primary-400 hover:text-primary-300 border border-primary-500/30 rounded-lg px-3 py-1.5 cursor-pointer disabled:opacity-50"
                          >
                            {predictingDiffs ? 'Discovering...' : 'Predict Differentiators'}
                          </button>
                        </div>
                        <p className="text-[10px] text-dark-600 mt-1">Customize these results — the differentiators we discover may not be the ones you consider most important.</p>

                        {/* Predict Differentiators progress */}
                        {predictDiffMessages.length > 0 && (
                          <div className="mt-3 space-y-1">
                            {predictDiffMessages.slice(-5).map((msg, i) => (
                              <div key={i} className="flex items-center gap-2 text-xs text-dark-500">
                                {predictingDiffs && i === predictDiffMessages.slice(-5).length - 1
                                  ? <div className="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse shrink-0" />
                                  : msg.startsWith('Error:')
                                    ? <div className="w-1.5 h-1.5 rounded-full bg-red-400 shrink-0" />
                                    : <svg className="w-3 h-3 text-accent-emerald shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                                }
                                <span className={msg.startsWith('Error:') ? 'text-red-400' : ''}>{msg}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Suggested Differentiators review */}
                        {suggestedDiffs.length > 0 && (
                          <div className="mt-4">
                            <div className="flex items-center justify-between mb-2">
                              <span className="text-xs font-medium text-dark-400">Suggested ({suggestedDiffs.length})</span>
                              <div className="flex gap-2">
                                <button onClick={() => setSuggestDiffSelected(new Set(suggestedDiffs.map((_, i) => i)))} className="text-[10px] text-dark-500 hover:text-white cursor-pointer">Select All</button>
                                <button
                                  onClick={addSelectedDifferentiators}
                                  disabled={suggestDiffSelected.size === 0}
                                  className={`text-xs px-3 py-1 rounded-lg cursor-pointer disabled:opacity-50 transition-all font-medium ${suggestDiffSelected.size > 0 ? 'bg-primary-600 text-white hover:bg-primary-500 animate-pulse' : 'bg-dark-700 text-dark-400'}`}
                                >
                                  Add Selected ({suggestDiffSelected.size})
                                </button>
                              </div>
                            </div>
                            <div className="space-y-1 max-h-64 overflow-y-auto">
                              {suggestedDiffs.map((sd, i) => (
                                <label key={i} className="flex items-center gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-2 cursor-pointer hover:border-dark-600">
                                  <input
                                    type="checkbox"
                                    checked={suggestDiffSelected.has(i)}
                                    onChange={() => {
                                      setSuggestDiffSelected(prev => {
                                        const next = new Set(prev)
                                        if (next.has(i)) next.delete(i); else next.add(i)
                                        return next
                                      })
                                    }}
                                    className="accent-primary-500"
                                  />
                                  <span className="flex-1 text-white text-xs">{sd}</span>
                                </label>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>

                {/* Section 5: Existing Presence (collapsible) */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-4 overflow-hidden">
                  <button
                    onClick={() => setBrandSections(p => ({ ...p, presence: !p.presence }))}
                    className="w-full flex items-center justify-between p-5 cursor-pointer"
                  >
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-semibold text-white">Existing Presence</h4>
                      {(brandPresenceComplete || brandPresenceForm.youtube_url || brandPresenceForm.subreddits || brandPresenceForm.review_site_urls
                          ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">Done</span>
                          : !brandSections.presence ? <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-medium">Needs review</span> : null
                      )}
                    </div>
                    <svg className={`w-4 h-4 text-dark-500 transition-transform ${brandSections.presence ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>
                  {brandSections.presence && (
                    <div className="px-6 pb-6 space-y-4">
                      <p className="text-[10px] text-dark-500">This section is optional. Provide what you can — not every brand has all of these channels. Check the box below when you've filled in everything relevant to you.</p>
                      <label className="flex items-start gap-3 bg-dark-800/30 border border-dark-700/50 rounded-lg p-3 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={brandPresenceComplete}
                          onChange={e => setBrandPresenceComplete(e.target.checked)}
                          className="accent-primary-500 mt-0.5"
                        />
                        <div>
                          <span className="text-white text-xs font-medium">Mark presence section as complete</span>
                          <p className="text-[10px] text-dark-600 mt-0.5">Check this when you've provided all presence info relevant to your brand.</p>
                        </div>
                      </label>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">YouTube Channel URL</label>
                        <input
                          type="text"
                          value={brandPresenceForm.youtube_url}
                          onChange={e => setBrandPresenceForm(p => ({ ...p, youtube_url: e.target.value }))}
                          placeholder="https://youtube.com/@channel"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Subreddits <span className="text-dark-600">(comma-separated)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.subreddits}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, subreddits: e.target.value }))}
                            placeholder="r/subreddit1, r/subreddit2"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Review Sites <span className="text-dark-600">(URLs)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.review_site_urls}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, review_site_urls: e.target.value }))}
                            placeholder="G2, Capterra URLs"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Social Profiles <span className="text-dark-600">(URLs)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.social_profiles}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, social_profiles: e.target.value }))}
                            placeholder="Twitter, LinkedIn URLs"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                        <div>
                          <label className="block text-xs text-dark-400 mb-1">Podcast Appearances <span className="text-dark-600">(URLs)</span></label>
                          <input
                            type="text"
                            value={brandPresenceForm.podcasts}
                            onChange={e => setBrandPresenceForm(p => ({ ...p, podcasts: e.target.value }))}
                            placeholder="Podcast episode URLs"
                            className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                          />
                        </div>
                      </div>
                      <div>
                        <label className="block text-xs text-dark-400 mb-1">Key Content Assets <span className="text-dark-600">(URLs)</span></label>
                        <input
                          type="text"
                          value={brandPresenceForm.content_assets}
                          onChange={e => setBrandPresenceForm(p => ({ ...p, content_assets: e.target.value }))}
                          placeholder="Blog posts, whitepapers, documentation URLs"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                      </div>
                    </div>
                  )}
                </div>

                </div>{/* close readOnly pointer-events wrapper */}

                {/* Bottom save bar */}
                {!readOnly && (
                  <div className="flex justify-end gap-3 mt-6">
                    {brandProfile && (
                      <button
                        onClick={() => setConfirmDeleteBrand(true)}
                        className="px-4 py-2 text-red-400 hover:text-red-300 text-sm cursor-pointer"
                      >
                        Delete Profile
                      </button>
                    )}
                    <button
                      onClick={saveBrandProfile}
                      disabled={brandSaving || !brandDomain.trim()}
                      className="px-6 py-2.5 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 cursor-pointer"
                    >
                      {brandSaving ? 'Saving...' : 'Save Brand Profile'}
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        )}


        {/* ===== VIDEO TAB ===== */}
        {activeTab === 'video' && (
          <div className="max-w-4xl mx-auto animate-fade-in">

            {/* Cross-report insights */}
            {crossInsights.filter(i => i.tab === 'video' && !dismissedInsights.has(i.message)).map(insight => (
              <div key={insight.message} className="flex items-center justify-between bg-primary-500/5 border border-primary-500/20 rounded-xl px-4 py-3 mb-4">
                <span className="text-dark-300 text-xs flex-1 mr-3">{insight.message}</span>
                <div className="flex items-center gap-2 shrink-0">
                  <button onClick={() => setActiveTab(insight.targetTab as typeof activeTab)} className="text-xs px-3 py-1 bg-primary-500/10 border border-primary-500/20 text-primary-400 rounded-lg hover:bg-primary-500/20 transition-all cursor-pointer">{insight.cta}</button>
                  <button onClick={() => setDismissedInsights(prev => new Set([...prev, insight.message]))} className="text-dark-600 hover:text-dark-400 cursor-pointer"><svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg></button>
                </div>
              </div>
            ))}

            {/* YouTube API key required notice */}
            {saasEnabled && user && !hasYouTubeKey && videoView === 'input' && (
              <div className="bg-amber-500/5 border border-amber-500/20 rounded-xl px-5 py-4 mb-4">
                <div className="flex items-start gap-3">
                  <svg className="w-5 h-5 text-amber-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" /></svg>
                  <div>
                    <p className="text-sm font-medium text-amber-300">YouTube API key required</p>
                    <p className="text-xs text-amber-400/80 mt-1">Add your YouTube Data API v3 key in <button onClick={() => window.location.hash = '#/settings'} className="underline hover:text-amber-300 cursor-pointer">Settings &rarr; API Keys</button> to enable Video Authority analysis.</p>
                  </div>
                </div>
              </div>
            )}

            {/* Input View */}
            {videoView === 'input' && readOnly && (
              <div className="text-center py-16 max-w-md mx-auto">
                <svg className="w-12 h-12 mx-auto text-dark-600 mb-4" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>
                <h3 className="text-white font-semibold text-lg mb-2">Discover Your YouTube Authority</h3>
                <p className="text-dark-400 text-sm">YouTube is cited in 16% of all LLM answers. See how your videos contribute to your brand's AI visibility and what to do to improve.</p>
              </div>
            )}
            {videoView === 'input' && !readOnly && (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <h2 className="text-2xl font-bold text-white">Video Authority Analyzer</h2>
                  {videoDomain && (
                    <span className="text-dark-400 text-sm">{videoDomain}</span>
                  )}
                </div>

                {/* Why video matters callout */}
                <div className="bg-gradient-to-r from-primary-600/10 to-purple-600/10 border border-primary-500/20 rounded-2xl p-5">
                  <div className="flex gap-4">
                    <div className="text-3xl shrink-0 mt-0.5">
                      <svg className="w-8 h-8 text-primary-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>
                    </div>
                    <div>
                      <p className="text-white font-semibold text-sm">YouTube is now the #1 social authority source for LLM answers</p>
                      <p className="text-dark-400 text-xs mt-1.5 leading-relaxed">
                        YouTube appears in 16% of all LLM answers — cited 40% more than Reddit and 18x more than Instagram.
                        Its share of social citations doubled from 19% to 39% in just 4 months.
                        But AI visibility works differently than human attention: views and subscribers don't predict LLM influence.
                        What matters is transcript quality, keyword clarity, and structural extractability.
                        This analyzer evaluates your video ecosystem through the lens of what LLMs actually ingest.
                      </p>
                    </div>
                  </div>
                </div>

                {/* YouTube bot-detection warning */}
                <div className="bg-amber-500/5 border border-amber-500/20 rounded-2xl p-5">
                  <div className="flex gap-4">
                    <div className="shrink-0 mt-0.5">
                      <svg className="w-6 h-6 text-amber-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" /></svg>
                    </div>
                    <div>
                      <p className="text-amber-300 font-semibold text-sm">Heads up: YouTube bot detection may block transcript fetching</p>
                      <p className="text-amber-400/80 text-xs mt-1.5 leading-relaxed">
                        YouTube aggressively blocks transcript requests from datacenter IPs (including this hosted instance), so analyses run from here often fail to retrieve captions and produce empty or incomplete reports.
                        The most reliable workaround is to <span className="font-semibold">self-host this tool on a machine with a residential IP</span> (your laptop, a home server, etc.) — the same code path works fine from a normal home connection.
                        Source available at <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noreferrer" className="underline hover:text-amber-300">github.com/metavert-io/llmopt</a>.
                      </p>
                    </div>
                  </div>
                </div>


                {/* Collapsible Settings */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
                  <button
                    onClick={() => setVideoSettingsOpen(prev => !prev)}
                    className="w-full flex items-center justify-between p-4 cursor-pointer hover:bg-dark-800/30 transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <svg className={`w-4 h-4 text-dark-400 transition-transform ${videoSettingsOpen ? 'rotate-90' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" /></svg>
                      <span className="text-white font-semibold text-sm">Analysis Settings</span>
                    </div>
                    {!videoSettingsOpen && (
                      <div className="flex items-center gap-3 text-xs text-dark-500">
                        {videoChannelURL && <span className="text-emerald-400">Channel set</span>}
                        {videoSearchTerms.length > 0 && <span>{videoSearchTerms.length} topics</span>}
                      </div>
                    )}
                  </button>

                  {videoSettingsOpen && (
                  <div className="px-6 pb-6 space-y-6">

                {/* YouTube Presence */}
                <div className="space-y-4">
                  <h3 className="text-dark-300 font-medium text-sm">Your YouTube Presence</h3>

                  <div>
                    <label className="block text-sm text-dark-400 mb-1">YouTube Channel URL</label>
                    <input
                      type="url"
                      value={videoChannelURL}
                      onChange={e => setVideoChannelURL(e.target.value)}
                      placeholder="https://youtube.com/@yourchannel"
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder:text-dark-600 focus:outline-none focus:border-primary-500/50"
                    />
                  </div>

                  <div>
                    <label className="block text-sm text-dark-400 mb-1">Individual Video URLs</label>
                    {videoURLs.map((vurl, i) => (
                      <div key={i} className="flex gap-2 mb-2">
                        <input
                          type="url"
                          value={vurl}
                          onChange={e => {
                            const updated = [...videoURLs]
                            updated[i] = e.target.value
                            setVideoURLs(updated)
                          }}
                          placeholder="https://youtube.com/watch?v=..."
                          className="flex-1 px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder:text-dark-600 focus:outline-none focus:border-primary-500/50 text-sm"
                        />
                        {videoURLs.length > 1 && (
                          <button
                            onClick={() => setVideoURLs(prev => prev.filter((_, j) => j !== i))}
                            className="px-2 text-dark-500 hover:text-red-400 transition-colors cursor-pointer"
                          >
                            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                          </button>
                        )}
                      </div>
                    ))}
                    <button
                      onClick={() => setVideoURLs(prev => [...prev, ''])}
                      className="text-xs text-primary-400 hover:text-primary-300 transition-colors cursor-pointer"
                    >
                      + Add Video URL
                    </button>
                  </div>

                  <div>
                    <label className="block text-sm text-dark-400 mb-1">
                      Key Topics / Search Terms
                      {videoSearchTerms.some(t => videoSearchTermSources.get(t) === 'optimization') && (
                        <span className="text-amber-400 text-xs ml-2">includes {videoSearchTerms.filter(t => videoSearchTermSources.get(t) === 'optimization').length} optimization {videoSearchTerms.filter(t => videoSearchTermSources.get(t) === 'optimization').length === 1 ? 'question' : 'questions'}</span>
                      )}
                    </label>
                    <div className="flex flex-wrap gap-2 mb-2">
                      {videoSearchTerms.map((term, i) => {
                        const isOpt = videoSearchTermSources.get(term) === 'optimization'
                        return (
                          <span key={i} className={`inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm border ${isOpt ? 'bg-amber-500/15 text-amber-300 border-amber-500/25' : 'bg-primary-500/20 text-primary-300 border-primary-500/30'}`}>
                            {isOpt && <span className="text-[10px] opacity-60 mr-0.5">Q</span>}
                            {term}
                            <button
                              onClick={() => {
                                setVideoSearchTerms(prev => prev.filter((_, j) => j !== i))
                                if (videoDomain) {
                                  apiFetch('/api/video/search-terms', {
                                    method: 'POST',
                                    headers: { 'Content-Type': 'application/json' },
                                    body: JSON.stringify({ domain: videoDomain, action: 'remove', term }),
                                  }).catch(() => {})
                                }
                              }}
                              className="hover:text-white transition-colors cursor-pointer"
                            >
                              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                            </button>
                          </span>
                        )
                      })}
                    </div>
                    <input
                      type="text"
                      value={videoSearchTermInput}
                      onChange={e => setVideoSearchTermInput(e.target.value)}
                      onKeyDown={e => {
                        if (e.key === 'Enter' && videoSearchTermInput.trim()) {
                          const term = videoSearchTermInput.trim()
                          setVideoSearchTerms(prev => [...prev, term])
                          setVideoSearchTermInput('')
                          if (videoDomain) {
                            apiFetch('/api/video/search-terms', {
                              method: 'POST',
                              headers: { 'Content-Type': 'application/json' },
                              body: JSON.stringify({ domain: videoDomain, action: 'add', term }),
                            }).catch(() => {})
                          }
                        }
                      }}
                      placeholder="Type a search term and press Enter..."
                      className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder:text-dark-600 focus:outline-none focus:border-primary-500/50 text-sm"
                    />
                  </div>

                  <label className="flex items-start gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={videoIncludeBrandSearches}
                      onChange={e => setVideoIncludeBrandSearches(e.target.checked)}
                      className="mt-0.5 accent-primary-500"
                    />
                    <span className="text-sm text-dark-300">
                      Auto-search brand name
                      <span className="block text-xs text-dark-500 mt-0.5">
                        Runs <span className="text-dark-400">{'"<brand> review"'}</span>, <span className="text-dark-400">{'"<brand> tutorial"'}</span>, and <span className="text-dark-400">{'"<brand> vs <competitor>"'}</span> queries. Turn off if your brand name is a common phrase (e.g. dance/theater terms) — it generates many false positives.
                      </span>
                    </span>
                  </label>
                </div>


                  </div>
                  )}
                </div>

                {/* Auto-Discovery Button */}
                <div className="flex items-center gap-4">
                  <button
                    onClick={videoDiscover}
                    disabled={!videoDomain.trim() || videoDiscovering}
                    className="flex-1 px-6 py-3 bg-primary-600 text-white rounded-xl font-medium hover:bg-primary-500 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                  >
                    {videoDiscovering ? (
                      <>
                        <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                        Discovering...
                      </>
                    ) : (
                      'Find Relevant YouTube Content'
                    )}
                  </button>
                </div>

                {/* Discovery error messages */}
                {videoMessages.length > 0 && videoView === 'input' && (
                  <div className="bg-red-500/10 border border-red-500/20 rounded-xl p-4">
                    {videoMessages.map((msg, i) => (
                      <p key={i} className="text-red-400 text-sm">{msg}</p>
                    ))}
                  </div>
                )}

                {/* Previous Analyses */}
                {(videoAnalysisList.filter(a => domainKey(a.domain) === domainKey(selectedDomain)).length > 0 || failedAnalyses.filter(f => f.feed_type === 'video' && domainKey(f.domain) === domainKey(selectedDomain)).length > 0) && (
                  <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                    <h3 className="text-white font-semibold mb-3">Previous Analyses</h3>
                    <div className="space-y-2">
                      {failedAnalyses.filter(f => f.feed_type === 'video' && domainKey(f.domain) === domainKey(selectedDomain)).map(f => (
                        <div key={f.id} className="px-4 py-3 bg-dark-800/50 border border-amber-500/30 rounded-xl flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" />
                            <span className="text-amber-400 text-xs">
                              Failed ({errorCodeLabel(f.error_code)}) &middot; {fmtDate(f.failed_at)}
                            </span>
                          </div>
                          <button onClick={() => dismissFailedAnalysis(f.id)} className="text-dark-500 hover:text-dark-300 text-xs cursor-pointer">Dismiss</button>
                        </div>
                      ))}
                      {videoAnalysisList.filter(a => domainKey(a.domain) === domainKey(selectedDomain)).map(a => (
                        <button
                          key={a.id}
                          onClick={() => loadVideoAnalysis(a.domain)}
                          className="w-full text-left px-4 py-3 bg-dark-800/50 border border-dark-700 rounded-xl hover:border-primary-500/30 transition-all cursor-pointer flex items-center justify-between"
                        >
                          <div className="flex items-center gap-2">
                            {isVideoAnalysisStale(a.generated_at, a.domain) && (
                              <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" title="New optimization questions since this analysis" />
                            )}
                            <span className="text-dark-500 text-xs">
                              {a.video_count} videos &middot; {fmtDate(a.generated_at)}
                            </span>
                          </div>
                          {a.overall_score != null && (
                            <span className={`text-sm font-bold ${scoreTextColor(a.overall_score)}`}>
                              {a.overall_score}
                            </span>
                          )}
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Discovering View — SSE progress */}
            {videoView === 'discovering' && (
              <div className="max-w-2xl mx-auto space-y-6">
                <h2 className="text-2xl font-bold text-white text-center">Discovering YouTube Content</h2>
                <p className="text-dark-400 text-center text-sm">Searching for videos related to {videoDomain}</p>

                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-3 max-h-64 overflow-y-auto">
                    {videoMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {msg.startsWith('Upstream API issue') || msg.startsWith('Error') ? (
                          <svg className="w-4 h-4 text-red-400 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" /></svg>
                        ) : i < videoMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                        ) : (
                          <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse ml-1 mr-1 shrink-0" />
                        )}
                        <span className={msg.startsWith('Upstream API issue') ? 'text-amber-400' : msg.startsWith('Error') ? 'text-red-400' : i < videoMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>{msg}</span>
                      </div>
                    ))}
                    {videoDiscovering && videoMessages.length === 0 && (
                      <div className="flex items-center gap-3 text-sm">
                        <div className="w-4 h-4 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                        <span className="text-dark-300">Connecting to YouTube API...</span>
                      </div>
                    )}
                    <div ref={videoMessagesEndRef} />
                  </div>
                </div>
              </div>
            )}

            {/* Review View — after discovery */}
            {videoView === 'review' && (() => {
              const tagDefs = [
                { key: 'own', label: 'Own Channel', color: 'emerald' },
                { key: 'direct_mention', label: 'Mentions', color: 'primary' },
                { key: 'competitor_comparison', label: 'Competitor', color: 'amber' },
                { key: 'category_content', label: 'Category', color: 'slate' },
              ] as const
              const recencyOpts = [
                { key: '30d' as const, label: '30 days' },
                { key: '90d' as const, label: '90 days' },
                { key: '1y' as const, label: '1 year' },
                { key: '3y' as const, label: '3 years' },
                { key: 'all' as const, label: 'All time' },
              ]
              // Logarithmic view slider helpers
              const logMax = videoViewCountMax > 0 ? Math.log10(videoViewCountMax) : 0
              const viewSliderToCount = (pct: number) => pct === 0 ? 0 : Math.round(Math.pow(10, (pct / 100) * logMax))
              const countToSlider = (count: number) => count === 0 || logMax === 0 ? 0 : (Math.log10(Math.max(1, count)) / logMax) * 100

              return (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => setVideoView('input')}
                    className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                    Back to inputs
                  </button>
                  <span className="text-dark-400 text-sm">
                    {filteredDiscoveredVideos.length} of {discoveredVideos.length} videos
                    {filteredDiscoveredVideos.length < discoveredVideos.length && ' (filtered)'}
                  </span>
                </div>

                {/* Filter Toolbar */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-4 space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-white font-semibold text-sm">Filter &amp; Prioritize</h3>
                    {/* Quick presets */}
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          // Select all own + top 50 third-party by views
                          const own = filteredDiscoveredVideos.filter(v => v.relevance_tag === 'own')
                          const thirdParty = filteredDiscoveredVideos
                            .filter(v => v.relevance_tag !== 'own')
                            .sort((a, b) => (b.view_count || 0) - (a.view_count || 0))
                            .slice(0, 50)
                          setSelectedVideoIds(new Set([...own, ...thirdParty].map(v => v.video_id)))
                        }}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-400 hover:text-white hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Own + Top 50
                      </button>
                      <button
                        onClick={() => {
                          // Top 25 by views across all filtered
                          const top = [...filteredDiscoveredVideos]
                            .sort((a, b) => (b.view_count || 0) - (a.view_count || 0))
                            .slice(0, 25)
                          setSelectedVideoIds(new Set(top.map(v => v.video_id)))
                        }}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-400 hover:text-white hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Top 25
                      </button>
                      <button
                        onClick={() => setSelectedVideoIds(new Set(filteredDiscoveredVideos.map(v => v.video_id)))}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-400 hover:text-white hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Select All
                      </button>
                      <button
                        onClick={() => setSelectedVideoIds(new Set())}
                        className="text-[11px] px-2.5 py-1 rounded-lg bg-dark-800 text-dark-500 hover:text-dark-300 hover:bg-dark-700 transition-colors cursor-pointer border border-dark-700"
                      >
                        Clear
                      </button>
                    </div>
                  </div>

                  {/* Relevance tag toggles */}
                  <div>
                    <label className="text-[11px] text-dark-500 uppercase tracking-wider mb-1.5 block">Content type</label>
                    <div className="flex flex-wrap gap-2">
                      {tagDefs.map(t => {
                        const count = videoTagCounts[t.key] || 0
                        const active = videoFilterTags.has(t.key)
                        const colorMap: Record<string, string> = {
                          emerald: active ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40' : 'bg-dark-800 text-dark-600 border-dark-700',
                          primary: active ? 'bg-primary-500/20 text-primary-300 border-primary-500/40' : 'bg-dark-800 text-dark-600 border-dark-700',
                          amber: active ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-dark-800 text-dark-600 border-dark-700',
                          slate: active ? 'bg-dark-700 text-dark-300 border-dark-600' : 'bg-dark-800 text-dark-600 border-dark-700',
                        }
                        return (
                          <button
                            key={t.key}
                            onClick={() => {
                              setVideoFilterTags(prev => {
                                const next = new Set(prev)
                                next.has(t.key) ? next.delete(t.key) : next.add(t.key)
                                return next
                              })
                            }}
                            className={`text-xs px-3 py-1.5 rounded-lg border cursor-pointer transition-all ${colorMap[t.color]}`}
                          >
                            {t.label} ({count})
                          </button>
                        )
                      })}
                    </div>
                  </div>

                  {/* View count + recency row */}
                  <div className="grid grid-cols-2 gap-4">
                    {/* Minimum views slider */}
                    <div>
                      <label className="text-[11px] text-dark-500 uppercase tracking-wider mb-1.5 block">
                        Min views: {videoFilterMinViews === 0 ? 'None' : videoFilterMinViews.toLocaleString()}
                      </label>
                      <input
                        type="range"
                        min={0}
                        max={100}
                        step={1}
                        value={countToSlider(videoFilterMinViews)}
                        onChange={e => setVideoFilterMinViews(viewSliderToCount(Number(e.target.value)))}
                        className="w-full accent-primary-500 h-1.5"
                      />
                      <div className="flex justify-between text-[10px] text-dark-600 mt-0.5">
                        <span>0</span>
                        <span>{videoViewCountMax > 0 ? videoViewCountMax.toLocaleString() : '—'}</span>
                      </div>
                    </div>

                    {/* Recency filter */}
                    <div>
                      <label className="text-[11px] text-dark-500 uppercase tracking-wider mb-1.5 block">Published within</label>
                      <div className="flex gap-1.5">
                        {recencyOpts.map(o => (
                          <button
                            key={o.key}
                            onClick={() => setVideoFilterRecency(o.key)}
                            className={`text-[11px] px-2 py-1.5 rounded-lg border cursor-pointer transition-all ${
                              videoFilterRecency === o.key
                                ? 'bg-primary-500/20 text-primary-300 border-primary-500/40'
                                : 'bg-dark-800 text-dark-600 border-dark-700 hover:text-dark-400'
                            }`}
                          >
                            {o.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>

                  {/* Research note */}
                  <p className="text-[11px] text-dark-600 leading-relaxed">
                    Note: View count is a rough proxy for LLM training data frequency, not a direct predictor.
                    Research shows AI citation is driven by transcript quality and structural extractability, not engagement metrics.
                    Own-channel and direct-mention videos tend to have the highest analysis value.
                  </p>
                </div>

                {/* Run Analysis — prominent CTA */}
                <div className="flex flex-col items-center gap-2">
                  <button
                    onClick={videoAnalyze}
                    disabled={selectedVideoIds.size === 0 || videoAnalyzing}
                    className="px-8 py-3 bg-gradient-to-r from-primary-600 to-primary-500 text-white rounded-xl font-semibold text-base hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-primary-500/20"
                  >
                    Run Video Authority Analysis ({selectedVideoIds.size} videos)
                  </button>
                  <span className="text-dark-500 text-xs">
                    {selectedVideoIds.size} of {filteredDiscoveredVideos.length} filtered videos selected{videoQuotaEstimate > 0 && ` · ~${videoQuotaEstimate} API calls`}
                  </span>
                  <label className="flex items-center gap-2 cursor-pointer mt-1">
                    <input
                      type="checkbox"
                      checked={videoAutoArchive}
                      onChange={e => setVideoAutoArchive(e.target.checked)}
                      className="w-3.5 h-3.5 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500/30 cursor-pointer"
                    />
                    <span className="text-dark-400 text-xs">Automatically archive incomplete YouTube recommendations in your to-do list</span>
                  </label>
                </div>

                {/* Video list */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-2 max-h-[50vh] overflow-y-auto">
                    {filteredDiscoveredVideos.length === 0 ? (
                      <p className="text-dark-500 text-sm text-center py-8">No videos match the current filters. Adjust filters above.</p>
                    ) : (
                      filteredDiscoveredVideos.map(v => (
                        <label
                          key={v.video_id}
                          className={`flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all ${
                            selectedVideoIds.has(v.video_id)
                              ? 'border-primary-500/30 bg-primary-500/5'
                              : 'border-dark-800 bg-dark-900/30'
                          }`}
                        >
                          <input
                            type="checkbox"
                            checked={selectedVideoIds.has(v.video_id)}
                            onChange={() => {
                              setSelectedVideoIds(prev => {
                                const next = new Set(prev)
                                next.has(v.video_id) ? next.delete(v.video_id) : next.add(v.video_id)
                                return next
                              })
                            }}
                            className="mt-1 accent-primary-500"
                          />
                          <div className="flex-1 min-w-0">
                            <div className="flex items-start justify-between gap-2">
                              <span className="text-white text-sm font-medium line-clamp-1">{v.title}</span>
                              <span className={`shrink-0 text-[10px] px-2 py-0.5 rounded-full border ${
                                v.relevance_tag === 'own' ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30' :
                                v.relevance_tag === 'direct_mention' ? 'bg-primary-500/20 text-primary-300 border-primary-500/30' :
                                v.relevance_tag === 'competitor_comparison' ? 'bg-amber-500/20 text-amber-300 border-amber-500/30' :
                                'bg-dark-700/50 text-dark-400 border-dark-700'
                              }`}>
                                {v.relevance_tag === 'own' ? 'Own Channel' :
                                 v.relevance_tag === 'direct_mention' ? 'Direct Mention' :
                                 v.relevance_tag === 'competitor_comparison' ? 'Competitor' :
                                 'Category'}
                              </span>
                            </div>
                            <div className="flex items-center gap-3 mt-1 text-xs text-dark-500">
                              <span>{v.channel_title}</span>
                              <span>{(v.view_count || 0).toLocaleString()} views</span>
                              <span>{fmtDate(v.published_at)}</span>
                            </div>
                          </div>
                        </label>
                      ))
                    )}
                  </div>
                </div>

              </div>
              )
            })()}

            {/* Running View */}
            {videoView === 'running' && (
              <div className="max-w-2xl mx-auto space-y-6">
                <h2 className="text-2xl font-bold text-white text-center">Analyzing Video Authority</h2>
                <p className="text-dark-400 text-center text-sm">Full authority analysis for {videoDomain}</p>

                <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                  <div className="space-y-3 max-h-64 overflow-y-auto">
                    {videoMessages.map((msg, i) => (
                      <div key={i} className="flex items-center gap-3 text-sm">
                        {msg.startsWith('Upstream API issue') || msg.startsWith('Error') ? (
                          <svg className="w-4 h-4 text-red-400 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" /></svg>
                        ) : i < videoMessages.length - 1 ? (
                          <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                        ) : (
                          <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse ml-1 mr-1 shrink-0" />
                        )}
                        <span className={msg.startsWith('Upstream API issue') ? 'text-amber-400' : msg.startsWith('Error') ? 'text-red-400' : i < videoMessages.length - 1 ? 'text-dark-500' : 'text-dark-300'}>{msg}</span>
                      </div>
                    ))}
                    {videoAnalyzing && videoMessages.length === 0 && (
                      <div className="flex items-center gap-3 text-sm">
                        <div className="w-4 h-4 border-2 border-dark-700 border-t-primary-500 rounded-full animate-spin" />
                        <span className="text-dark-300">Starting analysis...</span>
                      </div>
                    )}
                    <div ref={videoMessagesEndRef} />
                  </div>
                </div>

                {/* Stop / Back buttons */}
                <div className="flex justify-center gap-3">
                  {videoAnalyzing ? (
                    <button
                      onClick={videoAnalyzeStop}
                      className="px-6 py-2.5 text-sm text-red-400 border border-red-500/30 rounded-xl hover:bg-red-500/10 transition-colors cursor-pointer font-medium"
                    >
                      Stop Analysis
                    </button>
                  ) : (
                    <button
                      onClick={() => setVideoView('review')}
                      className="px-6 py-2.5 text-sm text-dark-300 border border-dark-700 rounded-xl hover:bg-dark-800 transition-colors cursor-pointer"
                    >
                      Back to Review
                    </button>
                  )}
                </div>
              </div>
            )}

            {/* Results Dashboard */}
            {videoView === 'results' && videoAnalysis && (
              <div className="space-y-6">
                {/* Header */}
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => { setVideoView('input'); setVideoAnalysis(null) }}
                    className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                    Re-run or Change Settings
                  </button>
                  <div className="flex items-center gap-3">
                    {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                      <button
                        onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                        className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                        title="Share this domain"
                      >
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                        Share
                      </button>
                    )}
                    <button
                      onClick={async () => {
                        // In shared mode, build details from pre-loaded video analysis data
                        if (sharedModeRef.current && videoAnalysis) {
                          const details: VideoDetail[] = videoAnalysis.videos.map(v => ({
                            video_id: v.video_id,
                            title: v.title,
                            transcript: v.transcript || '',
                            transcript_length: v.transcript?.length || 0,
                            assessment: null,
                          }))
                          setVideoDetails(details)
                          setVideoDetailsExpanded(new Set())
                          setVideoView('transcripts')
                          return
                        }
                        try {
                          const res = await apiFetch(`/api/video/analyses/${encodeURIComponent(videoAnalysis.domain)}/details`)
                          if (res.ok) {
                            const data = await res.json() as VideoDetail[]
                            setVideoDetails(data)
                            setVideoDetailsExpanded(new Set())
                            setVideoView('transcripts')
                          }
                        } catch (err) { console.error('Failed to load details:', err) }
                      }}
                      className="text-xs text-dark-300 border border-dark-700 rounded-lg px-3 py-1.5 hover:bg-dark-800 transition-colors cursor-pointer"
                    >
                      View Transcripts
                    </button>
                    {!readOnly && (
                      <>
                        <button
                          onClick={() => setConfirmDeleteVideoAnalysis(true)}
                          className="text-xs text-red-400 border border-red-500/30 rounded-lg px-3 py-1.5 hover:bg-red-500/10 transition-colors cursor-pointer"
                        >
                          Delete
                        </button>
                        <button
                          onClick={() => { setVideoView('review'); setDiscoveredVideos(videoAnalysis.videos) }}
                          className="text-xs text-primary-400 border border-primary-500/30 rounded-lg px-3 py-1.5 hover:bg-primary-500/10 transition-colors cursor-pointer"
                        >
                          Re-run Analysis
                        </button>
                      </>
                    )}
                  </div>
                </div>

                {/* Stale: new optimization questions since this analysis */}
                {videoStaleOptimizations.length > 0 && (
                  <div className="bg-amber-500/10 border border-amber-500/20 rounded-xl p-4 space-y-2">
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex items-center gap-2">
                        <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                        </svg>
                        <span className="text-amber-400 text-sm font-medium">
                          {videoStaleOptimizations.length} new optimization {videoStaleOptimizations.length === 1 ? 'question' : 'questions'} since this video analysis
                        </span>
                      </div>
                      {!readOnly && (
                        <button
                          onClick={() => { setVideoView('input'); setVideoAnalysis(null) }}
                          className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all cursor-pointer whitespace-nowrap shrink-0"
                        >
                          Re-run with New Questions
                        </button>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {videoStaleOptimizations.map(o => (
                        <span key={o.id} className="text-xs px-2 py-1 bg-amber-500/10 text-amber-300 border border-amber-500/20 rounded-md">
                          {o.question}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {/* Stale: YouTube channel changed since this analysis */}
                {!readOnly && brandProfile?.presence?.youtube_url && videoAnalysis.config.channel_url &&
                  brandProfile.presence.youtube_url.replace(/\/$/, '').toLowerCase() !== videoAnalysis.config.channel_url.replace(/\/$/, '').toLowerCase() && (
                  <div className="flex items-center justify-between gap-3 bg-amber-500/10 border border-amber-500/20 rounded-xl p-4">
                    <div className="flex items-center gap-2">
                      <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                      </svg>
                      <span className="text-amber-400 text-sm font-medium">YouTube channel has changed since this analysis</span>
                    </div>
                    <button
                      onClick={() => { setVideoView('input'); setVideoAnalysis(null) }}
                      className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all cursor-pointer whitespace-nowrap shrink-0"
                    >
                      Re-run Analysis
                    </button>
                  </div>
                )}

                {/* Unified 4-Pillar Report */}
                {videoAnalysis.result && (() => {
                  const r = videoAnalysis.result
                  const s = r.overall_score
                  const label = s >= 80 ? 'Strong Authority' : s >= 60 ? 'Moderate' : s >= 40 ? 'Weak' : 'Poor'
                  const bn = r.brand_narrative
                  const sent = bn?.sentiment
                  const sentTotal = sent ? sent.positive + sent.neutral + sent.negative : 0
                  const pctPos = sentTotal > 0 ? Math.round(sent!.positive / sentTotal * 100) : 0
                  const pctNeu = sentTotal > 0 ? Math.round(sent!.neutral / sentTotal * 100) : 0
                  const pctNeg = sentTotal > 0 ? Math.round(sent!.negative / sentTotal * 100) : 0
                  const sov = r.topical_dominance?.share_of_voice || []
                  const maxMentions = Math.max(...(sov.map(e => e.mention_count).concat([1])))
                  return (
                    <div className="space-y-6">

                      {/* Header + Score */}
                      <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                        <div className="flex items-center gap-5">
                          <div className={`w-16 h-16 rounded-xl flex items-center justify-center text-2xl font-bold shrink-0 border ${scoreBadge(s)}`}>{s}</div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-3 mb-0.5">
                              <h2 className="text-lg font-bold text-white truncate">{videoAnalysis.domain}</h2>
                              <span className={`text-xs font-medium whitespace-nowrap ${scoreTextColor(s)}`}>{label}</span>
                            </div>
                            <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-dark-500">
                              <span>{videoAnalysis.videos.length} videos</span>
                              <span>{new Date(videoAnalysis.generated_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}</span>
                              {videoAnalysis.brand_context_used && <span className="text-primary-400">Brand context</span>}
                              {r.video_scorecards?.length > 0 && (() => {
                                const withT = r.video_scorecards.filter(c => c.has_transcript).length
                                const tot = r.video_scorecards.length
                                return <span className={withT === tot ? 'text-emerald-400' : withT > 0 ? 'text-amber-400' : 'text-red-400'}>{withT}/{tot} transcripts</span>
                              })()}
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* 4 Pillar Dimension Cards */}
                      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                        {[
                          { name: 'Transcript Authority', weight: '30%', score: r.transcript_authority?.score || 0,
                            desc: 'Spoken keyword density, quotability, statistical evidence, information density in own channel content' },
                          { name: 'Topical Dominance', weight: '25%', score: r.topical_dominance?.score || 0,
                            desc: 'Topic coverage breadth/depth vs. competitors, share of voice, content gap opportunities' },
                          { name: 'Citation Network', weight: '25%', score: r.citation_network?.score || 0,
                            desc: 'Third-party creator mentions, structural authority of referencing channels, concentration risk' },
                          { name: 'Brand Narrative', weight: '20%', score: r.brand_narrative?.score || 0,
                            desc: 'Mention sentiment and extractability, narrative coherence, how LLMs would frame this brand' },
                        ].map(dim => (
                          <div key={dim.name} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                            <div className="flex items-center justify-between mb-3">
                              <div>
                                <h4 className="text-white font-medium text-sm">{dim.name}</h4>
                                <span className="text-dark-500 text-xs">{dim.weight} weight</span>
                              </div>
                              <span className={`text-lg font-bold ${scoreTextColor(dim.score)}`}>{dim.score}</span>
                            </div>
                            <div className="h-1.5 bg-dark-800 rounded-full mb-3 overflow-hidden">
                              <div className={`h-full rounded-full transition-all duration-700 ${scoreBgSolid(dim.score)}`} style={{ width: `${dim.score}%` }} />
                            </div>
                            <p className="text-dark-500 text-xs leading-relaxed">{dim.desc}</p>
                          </div>
                        ))}
                      </div>

                      {/* Summary */}
                      {r.executive_summary && (
                        <div className="bg-primary-500/5 border border-primary-500/20 rounded-2xl p-6">
                          <div className="flex items-center justify-between mb-3">
                            <h3 className="text-primary-300 font-semibold">Summary</h3>
                            <button onClick={() => setExpandedSections(prev => { const n = new Set(prev); n.has('video-summary') ? n.delete('video-summary') : n.add('video-summary'); return n })}
                              className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer">{expandedSections.has('video-summary') ? 'Collapse' : 'Expand'}</button>
                          </div>
                          <p className={`text-dark-300 text-sm leading-relaxed whitespace-pre-wrap ${!expandedSections.has('video-summary') ? 'line-clamp-3' : ''}`}>{r.executive_summary}</p>
                        </div>
                      )}

                      {/* Brand Recognition vs Discovery callout */}
                      <div className="bg-purple-500/5 border border-purple-500/20 rounded-xl px-5 py-3">
                        <p className="text-dark-400 text-xs leading-relaxed">
                          <span className="text-purple-400 font-medium">Brand Recognition vs. Discovery:</span>{' '}
                          This analysis covers both brand-specific content (reviews, tutorials) and category-level content where your audience
                          searches without knowing your brand. Videos tagged "category_content" represent discovery opportunities — roundups and
                          how-tos where your brand should appear.
                        </p>
                      </div>

                      {/* Narrative: What LLMs Believe */}
                      {bn?.narrative_summary && (
                        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                          <h3 className="text-white font-semibold mb-3">What LLMs Probably Believe About You</h3>
                          <p className="text-dark-300 text-sm leading-relaxed whitespace-pre-wrap">{bn.narrative_summary}</p>
                          {sentTotal > 0 && (
                            <div className="mt-4">
                              <div className="h-3 bg-dark-800 rounded-full mb-2 overflow-hidden flex">
                                {pctPos > 0 && <div className="bg-emerald-500 h-full" style={{ width: `${pctPos}%` }} />}
                                {pctNeu > 0 && <div className="bg-dark-600 h-full" style={{ width: `${pctNeu}%` }} />}
                                {pctNeg > 0 && <div className="bg-red-500 h-full" style={{ width: `${pctNeg}%` }} />}
                              </div>
                              <div className="flex gap-3 text-[10px]">
                                <span className="text-emerald-400">{pctPos}% Positive</span>
                                <span className="text-dark-500">{pctNeu}% Neutral</span>
                                <span className="text-red-400">{pctNeg}% Negative</span>
                                <span className="text-dark-600 ml-auto">{sentTotal} mentions</span>
                              </div>
                            </div>
                          )}
                        </div>
                      )}

                      {/* Key Themes */}
                      {bn?.key_themes?.length > 0 && (
                        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                          <h3 className="text-white font-semibold mb-3">Key Themes</h3>
                          <div className="flex flex-wrap gap-2">
                            {bn.key_themes.map((theme, i) => (
                              <span key={i} className="px-3 py-1.5 bg-dark-800 border border-dark-700 rounded-lg text-dark-300 text-sm">{theme}</span>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Share of Voice */}
                      {sov.length > 0 && (
                        <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                          <h3 className="text-white font-semibold mb-4">Share of Voice</h3>
                          <div className="space-y-3">
                            {sov.map((entry, i) => (
                              <div key={i}>
                                <div className="flex justify-between text-sm mb-1">
                                  <span className={i === 0 ? 'text-primary-300 font-medium' : 'text-dark-300'}>{entry.brand_name}</span>
                                  <span className="text-dark-400">{entry.mention_count} mentions ({entry.percentage.toFixed(1)}%)</span>
                                </div>
                                <div className="h-2.5 bg-dark-800 rounded-full overflow-hidden">
                                  <div className={`h-full rounded-full transition-all duration-700 ${i === 0 ? 'bg-primary-500' : 'bg-dark-600'}`}
                                    style={{ width: `${(entry.mention_count / maxMentions) * 100}%` }} />
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Top Creators */}
                      {r.citation_network?.top_creators?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Top Creators ({r.citation_network.top_creators.length})
                          </h3>
                          <div className="grid gap-3">
                            {r.citation_network.top_creators.map((c, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4 flex items-center gap-4">
                                {c.authority_score > 0 && (
                                  <span className={`text-lg font-bold shrink-0 w-10 text-center ${
                                    scoreTextColor(c.authority_score)
                                  }`}>{c.authority_score}</span>
                                )}
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2 mb-0.5">
                                    <p className="text-white font-medium text-sm truncate">{c.channel_title}</p>
                                    <span className={`shrink-0 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                      c.role === 'advocate' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                                      c.role === 'critic' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                      'bg-dark-700/50 text-dark-400 border border-dark-600'
                                    }`}>{c.role}</span>
                                  </div>
                                  <p className="text-dark-400 text-xs">{c.subscriber_count?.toLocaleString()} subs &middot; {c.video_count} videos &middot; {c.total_views?.toLocaleString()} views</p>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Brand Mentions */}
                      {bn?.brand_mentions?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Brand Mentions ({bn.brand_mentions.length})
                          </h3>
                          <div className="space-y-3">
                            {bn.brand_mentions.map((m, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                                <div className="flex items-start justify-between gap-2 mb-1">
                                  <span className="text-white text-sm font-medium line-clamp-1">{m.title}</span>
                                  <span className={`shrink-0 text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    m.sentiment === 'positive' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                                    m.sentiment === 'negative' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{m.sentiment}</span>
                                </div>
                                <p className="text-dark-400 text-xs mb-1">{m.channel_title} &middot; {m.view_count?.toLocaleString()} views</p>
                                {m.mention_context && <p className="text-dark-300 text-xs italic leading-relaxed">&ldquo;{m.mention_context}&rdquo;</p>}
                                <div className="flex gap-3 mt-1.5 text-[10px] text-dark-500">
                                  <span className="capitalize">Position: {m.mention_position}</span>
                                  <span className="capitalize">Extractability: {m.extractability}</span>
                                  {m.competitors_mentioned?.length > 0 && <span>Also mentions: {m.competitors_mentioned.join(', ')}</span>}
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Content Gaps */}
                      {r.topical_dominance?.content_gaps?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Content Gaps ({r.topical_dominance.content_gaps.length})
                          </h3>
                          <div className="space-y-3">
                            {r.topical_dominance.content_gaps.map((gap, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4 flex items-center gap-4">
                                <span className={`text-lg font-bold shrink-0 w-10 text-center ${
                                  scoreTextColor(gap.opportunity_score)
                                }`}>{gap.opportunity_score}</span>
                                <div className="min-w-0">
                                  <p className="text-white font-medium text-sm">{gap.query}</p>
                                  <p className="text-dark-500 text-xs">{gap.video_count} videos &middot; Competitors: {gap.competitors_mentioned?.join(', ') || 'none'}</p>
                                  {gap.recommendation && <p className="text-dark-400 text-xs mt-0.5">{gap.recommendation}</p>}
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Creator Targets */}
                      {r.citation_network?.creator_targets?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Creator Targeting Opportunities ({r.citation_network.creator_targets.length})
                          </h3>
                          <div className="grid gap-3">
                            {r.citation_network.creator_targets.map((ct, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                                <div className="flex items-center justify-between mb-1">
                                  <span className="text-white text-sm font-medium">{ct.channel_title}</span>
                                  <span className="text-dark-500 text-xs">{ct.subscriber_count?.toLocaleString()} subs</span>
                                </div>
                                <p className="text-dark-400 text-xs mb-1">{ct.category_relevance}</p>
                                {ct.competitors_mentioned?.length > 0 && <p className="text-dark-500 text-xs">Covers: {ct.competitors_mentioned.join(', ')}</p>}
                                {ct.outreach_reason && <p className="text-primary-300/80 text-xs mt-1">{ct.outreach_reason}</p>}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Per-Video Scorecards */}
                      {r.video_scorecards?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Video Scorecards ({r.video_scorecards.length})
                          </h3>
                          <div className="space-y-3">
                          {r.video_scorecards.map(card => {
                            const expanded = videoExpandedCards.has(card.video_id)
                            return (
                              <div key={card.video_id} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
                                <button
                                  onClick={() => setVideoExpandedCards(prev => {
                                    const next = new Set(prev)
                                    next.has(card.video_id) ? next.delete(card.video_id) : next.add(card.video_id)
                                    return next
                                  })}
                                  className="w-full px-5 py-3.5 flex items-center justify-between cursor-pointer hover:bg-dark-800/30 transition-colors"
                                >
                                  <div className="flex items-center gap-3 min-w-0">
                                    <span className={`text-lg font-bold ${
                                      scoreTextColor(card.overall_score)
                                    }`}>{card.overall_score}</span>
                                    <div className="min-w-0">
                                      <span className="text-white text-sm font-medium truncate block">{card.title}</span>
                                      {!card.has_transcript && <span className="text-red-400 text-[10px]">No transcript — scores capped</span>}
                                    </div>
                                  </div>
                                  <svg className={`w-4 h-4 text-dark-500 transition-transform shrink-0 ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                </button>
                                {expanded && (
                                  <div className="px-5 pb-4 space-y-3">
                                    <div className="grid grid-cols-3 gap-3">
                                      {[
                                        { label: 'Transcript Power', score: card.transcript_power },
                                        { label: 'Structural Extractability', score: card.structural_extractability },
                                        { label: 'Discovery Surface', score: card.discovery_surface },
                                      ].map(d => (
                                        <div key={d.label}>
                                          <div className="flex justify-between text-xs mb-1">
                                            <span className="text-dark-400">{d.label}</span>
                                            <span className={`font-medium ${scoreTextColor(d.score)}`}>{d.score}</span>
                                          </div>
                                          <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                                            <div className={`h-full rounded-full transition-all duration-700 ${scoreBgSolid(d.score)}`} style={{ width: `${d.score}%` }} />
                                          </div>
                                        </div>
                                      ))}
                                    </div>
                                    {card.key_findings?.length > 0 && (
                                      <div>
                                        <p className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-1.5">Key Findings</p>
                                        <ul className="space-y-1">
                                          {card.key_findings.map((f, i) => (
                                            <li key={i} className="text-dark-300 text-xs leading-relaxed flex gap-2">
                                              <span className="text-dark-600 shrink-0">-</span><span>{f}</span>
                                            </li>
                                          ))}
                                        </ul>
                                      </div>
                                    )}
                                  </div>
                                )}
                              </div>
                            )
                          })}
                          </div>
                        </div>
                      )}

                      {/* Recommendations */}
                      {r.recommendations?.length > 0 && (
                        <div>
                          <h3 className="text-xs font-semibold text-dark-500 uppercase tracking-widest mb-4">
                            Recommendations ({r.recommendations.length})
                          </h3>
                          <div className="space-y-3">
                            {r.recommendations.map((rec, i) => (
                              <div key={i} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-4">
                                <div className="flex items-center gap-2 mb-2">
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider ${
                                    rec.priority === 'high' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                                    rec.priority === 'medium' ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30' :
                                    'bg-dark-700/50 text-dark-400 border border-dark-600'
                                  }`}>{rec.priority}</span>
                                  <span className="text-dark-500 text-xs capitalize">{rec.dimension?.replace(/_/g, ' ')}</span>
                                </div>
                                <p className="text-white text-sm font-medium mb-1">{rec.action}</p>
                                <p className="text-dark-400 text-xs">{rec.expected_impact}</p>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Confidence Note */}
                      {r.confidence_note && (
                        <div className="bg-dark-900/30 border border-dark-800/50 rounded-xl p-4">
                          <p className="text-dark-500 text-xs leading-relaxed italic">{r.confidence_note}</p>
                        </div>
                      )}
                    </div>
                  )
                })()}
              </div>
            )}

            {/* Transcript & Assessment Viewer */}
            {videoView === 'transcripts' && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <button
                    onClick={() => setVideoView('results')}
                    className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                    Back to Results
                  </button>
                  <span className="text-dark-500 text-xs">{videoDetails.length} videos — {videoDetails.filter(d => d.transcript).length} with transcripts, {videoDetails.filter(d => d.assessment).length} assessed</span>
                </div>

                {videoDetails.map(detail => {
                  const expanded = videoDetailsExpanded.has(detail.video_id)
                  const hasT = !!detail.transcript
                  const hasA = !!detail.assessment
                  return (
                    <div key={detail.video_id} className="bg-dark-900/50 border border-dark-800 rounded-2xl overflow-hidden">
                      <button
                        onClick={() => setVideoDetailsExpanded(prev => {
                          const next = new Set(prev)
                          next.has(detail.video_id) ? next.delete(detail.video_id) : next.add(detail.video_id)
                          return next
                        })}
                        className="w-full px-5 py-3.5 flex items-center justify-between cursor-pointer hover:bg-dark-800/30 transition-colors"
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <span className="text-white text-sm font-medium truncate">{detail.title}</span>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          {hasA && <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary-500/15 text-primary-400 border border-primary-500/20">Assessed</span>}
                          {hasT ? (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/15 text-emerald-400 border border-emerald-500/20">Transcript</span>
                          ) : (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-dark-700/50 text-dark-500 border border-dark-600">No transcript</span>
                          )}
                          <svg className={`w-4 h-4 text-dark-500 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                        </div>
                      </button>
                      {expanded && (
                        <div className="px-5 pb-4 space-y-4">
                          {hasA && detail.assessment && (
                            <div className="bg-primary-500/5 border border-primary-500/20 rounded-xl p-4 space-y-3">
                              <h4 className="text-primary-300 text-xs font-semibold uppercase tracking-wider">Assessment</h4>
                              <div className="grid grid-cols-3 gap-3">
                                {[
                                  { label: 'Keyword Alignment', val: detail.assessment.keyword_alignment },
                                  { label: 'Quotability', val: detail.assessment.quotability },
                                  { label: 'Info Density', val: detail.assessment.info_density },
                                ].map(m => (
                                  <div key={m.label}>
                                    <div className="flex justify-between text-xs mb-1">
                                      <span className="text-dark-400">{m.label}</span>
                                      <span className={`font-medium ${m.val >= 70 ? 'text-emerald-400' : m.val >= 40 ? 'text-amber-400' : 'text-red-400'}`}>{m.val}</span>
                                    </div>
                                    <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                                      <div className={`h-full rounded-full ${m.val >= 70 ? 'bg-emerald-500' : m.val >= 40 ? 'bg-amber-500' : 'bg-red-500'}`} style={{ width: `${m.val}%` }} />
                                    </div>
                                  </div>
                                ))}
                              </div>
                              <p className="text-dark-300 text-xs">{detail.assessment.summary}</p>
                              {detail.assessment.brand_sentiment !== 'none' && (
                                <p className="text-dark-400 text-xs">Brand sentiment: <span className={detail.assessment.brand_sentiment === 'positive' ? 'text-emerald-400' : detail.assessment.brand_sentiment === 'negative' ? 'text-red-400' : 'text-dark-300'}>{detail.assessment.brand_sentiment}</span></p>
                              )}
                              {detail.assessment.topics?.length > 0 && (
                                <div className="flex flex-wrap gap-1.5">
                                  {detail.assessment.topics.map((t, i) => (
                                    <span key={i} className="px-2 py-0.5 bg-dark-800 border border-dark-700 rounded text-dark-300 text-[10px]">{t}</span>
                                  ))}
                                </div>
                              )}
                              {detail.assessment.key_quotes?.length > 0 && (
                                <div className="space-y-1.5">
                                  <p className="text-dark-500 text-[10px] font-semibold uppercase tracking-wider">Key Quotes</p>
                                  {detail.assessment.key_quotes.map((q, i) => (
                                    <p key={i} className="text-dark-300 text-xs italic border-l-2 border-primary-500/30 pl-3">"{q}"</p>
                                  ))}
                                </div>
                              )}
                            </div>
                          )}
                          {hasT ? (
                            <div>
                              <h4 className="text-dark-500 text-xs font-semibold uppercase tracking-wider mb-2">Transcript ({(detail.transcript_length || detail.transcript.length).toLocaleString()} chars)</h4>
                              <pre className="text-dark-300 text-xs leading-relaxed whitespace-pre-wrap bg-dark-950 border border-dark-800 rounded-xl p-4 max-h-48 overflow-y-auto font-sans">
                                {detail.transcript}{detail.transcript_length > detail.transcript.length ? '…' : ''}
                              </pre>
                              {detail.transcript_length > detail.transcript.length && (
                                <p className="text-dark-600 text-[10px] mt-1 italic">Showing first {detail.transcript.length.toLocaleString()} of {detail.transcript_length.toLocaleString()} chars</p>
                              )}
                            </div>
                          ) : (
                            <p className="text-dark-500 text-xs italic">No transcript available for this video.</p>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )}

        {/* ─── Reddit Tab ──────────────────────────────────────── */}
        {activeTab === 'reddit' && (
          <div className="max-w-4xl mx-auto animate-fade-in">

            {/* Cross-report insights */}
            {crossInsights.filter(i => i.tab === 'reddit' && !dismissedInsights.has(i.message)).map(insight => (
              <div key={insight.message} className="flex items-center justify-between bg-primary-500/5 border border-primary-500/20 rounded-xl px-4 py-3 mb-4">
                <span className="text-dark-300 text-xs flex-1 mr-3">{insight.message}</span>
                <div className="flex items-center gap-2 shrink-0">
                  <button onClick={() => setActiveTab(insight.targetTab as typeof activeTab)} className="text-xs px-3 py-1 bg-primary-500/10 border border-primary-500/20 text-primary-400 rounded-lg hover:bg-primary-500/20 transition-all cursor-pointer">{insight.cta}</button>
                  <button onClick={() => setDismissedInsights(prev => new Set([...prev, insight.message]))} className="text-dark-600 hover:text-dark-400 cursor-pointer"><svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg></button>
                </div>
              </div>
            ))}

            {/* Input View */}
            {redditView === 'input' && readOnly && (
              <div className="text-center py-16 max-w-md mx-auto">
                <svg className="w-12 h-12 mx-auto text-dark-600 mb-4" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" /></svg>
                <h3 className="text-white font-semibold text-lg mb-2">Discover Your Reddit Authority</h3>
                <p className="text-dark-400 text-sm">Reddit is a top training data source for every major LLM. See what people say about your brand and how it shapes what AI recommends.</p>
              </div>
            )}

            {redditView === 'input' && !readOnly && (
              <>
                {/* Domain auto-populated — show as label */}
                <div className="mb-6">
                  <div className="flex items-center gap-3 mb-1">
                    <h2 className="text-xl font-bold text-white">Reddit Authority Analyzer</h2>
                  </div>
                  <p className="text-dark-400 text-sm">Analyze Reddit discussions to assess how Reddit influences LLMs to recommend your brand.</p>
                </div>

                {/* Settings panel */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl mb-6">
                  <button
                    onClick={() => setRedditSettingsOpen(!redditSettingsOpen)}
                    className="w-full px-5 py-4 flex items-center justify-between cursor-pointer"
                  >
                    <span className="text-white font-medium text-sm">Settings</span>
                    <svg className={`w-4 h-4 text-dark-400 transition-transform ${redditSettingsOpen ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                  </button>

                  {redditSettingsOpen && (
                    <div className="px-5 pb-5 space-y-5 border-t border-dark-800 pt-4">
                      {/* Subreddits */}
                      <div>
                        <label className="block text-sm text-dark-400 mb-1">Subreddits to Search</label>
                        <div className="flex flex-wrap gap-2 mb-2">
                          {redditSubreddits.map((sub, i) => (
                            <span key={i} className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm border bg-orange-500/15 text-orange-300 border-orange-500/25">
                              r/{sub}
                              <button onClick={() => setRedditSubreddits(prev => prev.filter((_, j) => j !== i))} className="ml-0.5 hover:text-white cursor-pointer">
                                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                              </button>
                            </span>
                          ))}
                        </div>
                        <input
                          type="text"
                          value={redditSubredditInput}
                          onChange={e => setRedditSubredditInput(e.target.value)}
                          onKeyDown={e => {
                            if (e.key === 'Enter' && redditSubredditInput.trim()) {
                              e.preventDefault()
                              const sub = redditSubredditInput.trim().replace(/^\/?(r\/)?/, '')
                              if (sub && !redditSubreddits.includes(sub)) {
                                setRedditSubreddits(prev => [...prev, sub])
                              }
                              setRedditSubredditInput('')
                            }
                          }}
                          placeholder="Add subreddit (e.g. technology) and press Enter"
                          className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                        />
                        {redditSubreddits.length === 0 && (
                          <p className="text-dark-500 text-xs mt-1">No specific subreddits — will search all of Reddit</p>
                        )}
                      </div>

                      {/* Time filter */}
                      <div>
                        <label className="block text-sm text-dark-400 mb-1">Time Range</label>
                        <div className="flex gap-2">
                          {([['month', 'Past Month'], ['year', 'Past Year'], ['all', 'All Time']] as const).map(([value, label]) => (
                            <button
                              key={value}
                              onClick={() => setRedditTimeFilter(value)}
                              className={`px-3 py-1.5 text-xs rounded-lg border cursor-pointer transition-colors ${
                                redditTimeFilter === value
                                  ? 'bg-primary-600/20 border-primary-500/40 text-primary-300'
                                  : 'bg-dark-800 border-dark-700 text-dark-400 hover:text-white'
                              }`}
                            >
                              {label}
                            </button>
                          ))}
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                {/* Search Terms */}
                <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5 mb-6">
                  <label className="block text-sm text-dark-400 mb-1">
                    Key Topics / Search Terms
                    {redditSearchTerms.some(t => redditSearchTermSources.get(t) === 'optimization') && (
                      <span className="text-amber-400 text-xs ml-2">
                        includes {redditSearchTerms.filter(t => redditSearchTermSources.get(t) === 'optimization').length} optimization questions
                      </span>
                    )}
                  </label>
                  <div className="flex flex-wrap gap-2 mb-2">
                    {redditSearchTerms.map((term, i) => {
                      const isOpt = redditSearchTermSources.get(term) === 'optimization'
                      return (
                        <span key={i} className={`inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm border ${
                          isOpt ? 'bg-amber-500/15 text-amber-300 border-amber-500/25' : 'bg-primary-500/20 text-primary-300 border-primary-500/30'
                        }`}>
                          {isOpt && <span className="text-[10px] opacity-60 mr-0.5">Q</span>}
                          {term}
                          <button onClick={() => {
                            setRedditSearchTerms(prev => prev.filter((_, j) => j !== i))
                            setRedditSearchTermSources(prev => { const m = new Map(prev); m.delete(term); return m })
                          }} className="ml-0.5 hover:text-white cursor-pointer">
                            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                          </button>
                        </span>
                      )
                    })}
                  </div>
                  <input
                    type="text"
                    value={redditSearchTermInput}
                    onChange={e => setRedditSearchTermInput(e.target.value)}
                    onKeyDown={e => {
                      if (e.key === 'Enter' && redditSearchTermInput.trim()) {
                        e.preventDefault()
                        const term = redditSearchTermInput.trim()
                        if (!redditSearchTerms.map(t => t.toLowerCase()).includes(term.toLowerCase())) {
                          setRedditSearchTerms(prev => [...prev, term])
                        }
                        setRedditSearchTermInput('')
                      }
                    }}
                    placeholder="Add a search term and press Enter"
                    className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 text-sm"
                  />
                </div>

                {/* Discover button */}
                <button
                  onClick={redditDiscover}
                  disabled={redditSearchTerms.length === 0 || redditDiscovering}
                  className="w-full py-3 bg-gradient-to-r from-orange-600 to-orange-500 text-white font-semibold rounded-xl hover:from-orange-500 hover:to-orange-400 transition-all disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer text-sm"
                >
                  {redditDiscovering ? 'Discovering...' : 'Discover Reddit Threads'}
                </button>

                {/* Previous analyses */}
                {(redditAnalysisList.length > 0 || failedAnalyses.filter(f => f.feed_type === 'reddit').length > 0) && (
                  <div className="mt-8">
                    <h3 className="text-sm font-medium text-dark-400 mb-3">Previous Analyses</h3>
                    <div className="space-y-2">
                      {failedAnalyses.filter(f => f.feed_type === 'reddit').map(f => (
                        <div key={f.id} className="px-4 py-3 bg-dark-900/50 border border-amber-500/30 rounded-xl flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" />
                            <span className="text-amber-400 text-xs">
                              {f.domain} &middot; Failed ({errorCodeLabel(f.error_code)}) &middot; {fmtDate(f.failed_at)}
                            </span>
                          </div>
                          <button onClick={() => dismissFailedAnalysis(f.id)} className="text-dark-500 hover:text-dark-300 text-xs cursor-pointer">Dismiss</button>
                        </div>
                      ))}
                      {redditAnalysisList.map(a => (
                        <button
                          key={a.id}
                          onClick={() => loadRedditAnalysis(a.domain)}
                          className="w-full text-left px-4 py-3 bg-dark-900/50 border border-dark-800 rounded-xl hover:border-primary-500/40 transition-colors cursor-pointer"
                        >
                          <div className="flex items-center justify-between">
                            <span className="text-white text-sm font-medium">{a.domain}</span>
                            {a.overall_score != null && (
                              <span className={`text-sm font-bold ${scoreTextColor(a.overall_score)}`}>
                                {a.overall_score}
                              </span>
                            )}
                          </div>
                          <div className="flex items-center gap-2">
                            {isRedditAnalysisStale(a.generated_at, a.domain) && (
                              <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" title="New optimization questions since this analysis" />
                            )}
                            <span className="text-dark-500 text-xs">{a.thread_count} threads · {fmtDate(a.generated_at)}</span>
                          </div>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}

            {/* Discovering View */}
            {redditView === 'discovering' && (
              <div className="py-8">
                <div className="flex items-center gap-3 mb-6">
                  <div className="w-5 h-5 border-2 border-orange-400 border-t-transparent rounded-full animate-spin" />
                  <span className="text-white font-medium">Discovering Reddit threads...</span>
                </div>
                <div className="space-y-1.5 max-h-64 overflow-y-auto">
                  {redditMessages.map((msg, i) => (
                    <p key={i} className={`text-sm ${msg.startsWith('Upstream API issue') ? 'text-amber-400' : msg.startsWith('Error') ? 'text-red-400' : 'text-dark-400'}`}>{msg}</p>
                  ))}
                  <div ref={redditMessagesEndRef} />
                </div>
              </div>
            )}

            {/* Review View — show discovered threads */}
            {redditView === 'review' && (
              <div>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-lg font-bold text-white">
                    {discoveredThreads.length} Threads Found
                  </h2>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => { setRedditView('input'); setDiscoveredThreads([]); setSelectedThreadIds(new Set()) }}
                      className="px-3 py-1.5 text-xs text-dark-400 border border-dark-700 rounded-lg hover:text-white cursor-pointer"
                    >
                      Back
                    </button>
                    <button
                      onClick={() => setSelectedThreadIds(prev => prev.size === discoveredThreads.length ? new Set() : new Set(discoveredThreads.map(t => t.id)))}
                      className="px-3 py-1.5 text-xs text-dark-400 border border-dark-700 rounded-lg hover:text-white cursor-pointer"
                    >
                      {selectedThreadIds.size === discoveredThreads.length ? 'Deselect All' : 'Select All'}
                    </button>
                  </div>
                </div>

                {discoveredSubreddits.length > 0 && (
                  <div className="flex items-start gap-3 p-3 mb-4 bg-orange-500/5 border border-orange-500/20 rounded-xl">
                    <svg className="w-4 h-4 text-orange-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-orange-300 font-medium mb-1">Discovered new subreddits</p>
                      <div className="flex flex-wrap gap-1.5 mb-2">
                        {discoveredSubreddits.map(sub => (
                          <span key={sub} className="text-xs px-2 py-0.5 bg-orange-500/15 text-orange-300 border border-orange-500/20 rounded-full">r/{sub}</span>
                        ))}
                      </div>
                      <button
                        onClick={async () => {
                          const newSubs = [...redditSubreddits]
                          for (const sub of discoveredSubreddits) {
                            if (!newSubs.some(s => s.toLowerCase() === sub.toLowerCase())) newSubs.push(sub)
                          }
                          setRedditSubreddits(newSubs)
                          try {
                            await apiFetch(`/api/brands/${encodeURIComponent(redditDomain.trim())}/subreddits`, {
                              method: 'PATCH',
                              headers: { 'Content-Type': 'application/json' },
                              body: JSON.stringify({ subreddits: newSubs }),
                            })
                          } catch { /* ignore */ }
                          setDiscoveredSubreddits([])
                        }}
                        className="text-xs text-orange-400 hover:text-orange-300 underline underline-offset-2 cursor-pointer"
                      >
                        Add to brand profile
                      </button>
                    </div>
                  </div>
                )}

                <div className="space-y-2 max-h-[60vh] overflow-y-auto mb-6">
                  {discoveredThreads.map(t => {
                    const selected = selectedThreadIds.has(t.id)
                    return (
                      <div
                        key={t.id}
                        onClick={() => setSelectedThreadIds(prev => {
                          const s = new Set(prev)
                          if (s.has(t.id)) s.delete(t.id); else s.add(t.id)
                          return s
                        })}
                        className={`px-4 py-3 rounded-xl border cursor-pointer transition-colors ${
                          selected
                            ? 'bg-orange-500/10 border-orange-500/30'
                            : 'bg-dark-900/50 border-dark-800 hover:border-dark-700'
                        }`}
                      >
                        <div className="flex items-start gap-3">
                          <div className={`w-4 h-4 rounded border mt-0.5 flex items-center justify-center shrink-0 ${selected ? 'bg-orange-500 border-orange-500' : 'border-dark-600'}`}>
                            {selected && <svg className="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>}
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-0.5">
                              <span className="text-xs px-1.5 py-0.5 bg-orange-500/15 text-orange-300 border border-orange-500/20 rounded">r/{t.subreddit}</span>
                              <span className="text-dark-500 text-xs">{t.score} pts · {t.num_comments} comments</span>
                              <span className="text-dark-600 text-xs">· {fmtDate(t.created_utc)}</span>
                            </div>
                            <p className="text-white text-sm">{t.title}</p>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>

                <button
                  onClick={redditAnalyze}
                  disabled={selectedThreadIds.size === 0}
                  className="w-full py-3 bg-gradient-to-r from-orange-600 to-orange-500 text-white font-semibold rounded-xl hover:from-orange-500 hover:to-orange-400 transition-all disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer text-sm"
                >
                  Analyze {selectedThreadIds.size} Thread{selectedThreadIds.size !== 1 ? 's' : ''}
                </button>
                <label className="flex items-center gap-2 cursor-pointer mt-2">
                  <input
                    type="checkbox"
                    checked={redditAutoArchive}
                    onChange={e => setRedditAutoArchive(e.target.checked)}
                    className="w-3.5 h-3.5 rounded border-dark-600 bg-dark-800 text-orange-500 focus:ring-orange-500/30 cursor-pointer"
                  />
                  <span className="text-dark-400 text-xs">Archive to-dos for previous Reddit findings</span>
                </label>
              </div>
            )}

            {/* Running View */}
            {redditView === 'running' && (
              <div className="py-8">
                <div className="flex items-center justify-between mb-6">
                  <div className="flex items-center gap-3">
                    <div className="w-5 h-5 border-2 border-orange-400 border-t-transparent rounded-full animate-spin" />
                    <span className="text-white font-medium">Analyzing Reddit threads...</span>
                  </div>
                  <button onClick={redditAnalyzeStop} className="px-3 py-1.5 text-xs text-dark-400 border border-dark-700 rounded-lg hover:text-white cursor-pointer">
                    Stop
                  </button>
                </div>
                <div className="space-y-1.5 max-h-64 overflow-y-auto">
                  {redditMessages.map((msg, i) => (
                    <p key={i} className={`text-sm ${msg.startsWith('Upstream API issue') ? 'text-amber-400' : msg.startsWith('Error') ? 'text-red-400' : i === redditMessages.length - 1 ? 'text-white' : 'text-dark-500'}`}>{msg}</p>
                  ))}
                  <div ref={redditMessagesEndRef} />
                </div>
              </div>
            )}

            {/* Results View */}
            {redditView === 'results' && redditAnalysis?.result && (() => {
              const r = redditAnalysis.result
              const barColor = (s: number) => s >= 80 ? 'bg-emerald-400' : s >= 60 ? 'bg-amber-400' : s >= 40 ? 'bg-orange-400' : 'bg-red-400'

              return (
                <div className="space-y-6">
                  {/* Header — back button + actions (matches YouTube pattern) */}
                  <div className="flex items-center justify-between">
                    <button
                      onClick={() => { setRedditView('input'); setRedditAnalysis(null) }}
                      className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                    >
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                      Re-run or Change Settings
                    </button>
                    <div className="flex items-center gap-3">
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button
                          onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0-12.814a2.25 2.25 0 1 0 0-2.186m0 2.186a2.25 2.25 0 1 0 0 2.186" /></svg>
                          Share
                        </button>
                      )}
                      {!readOnly && (
                        <>
                          <button
                            onClick={() => setConfirmDeleteRedditAnalysis(true)}
                            className="text-xs text-red-400 border border-red-500/30 rounded-lg px-3 py-1.5 hover:bg-red-500/10 transition-colors cursor-pointer"
                          >
                            Delete
                          </button>
                          <button
                            onClick={() => { setRedditView('input'); setRedditAnalysis(null) }}
                            className="text-xs text-primary-400 border border-primary-500/30 rounded-lg px-3 py-1.5 hover:bg-primary-500/10 transition-colors cursor-pointer"
                          >
                            Re-run Analysis
                          </button>
                        </>
                      )}
                    </div>
                  </div>

                  {/* Staleness banner */}
                  {redditStaleOptimizations.length > 0 && (
                    <div className="bg-amber-500/10 border border-amber-500/20 rounded-xl p-4 space-y-2">
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-2">
                          <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                          <span className="text-amber-400 text-sm font-medium">
                            {redditStaleOptimizations.length} new optimization {redditStaleOptimizations.length === 1 ? 'question' : 'questions'} since this Reddit analysis
                          </span>
                        </div>
                        {!readOnly && (
                          <button onClick={() => { setRedditView('input'); setRedditAnalysis(null) }} className="px-3 py-1.5 text-xs font-medium text-amber-400 border border-amber-500/30 rounded-lg hover:bg-amber-500/10 transition-colors cursor-pointer whitespace-nowrap">
                            Re-run with New Questions
                          </button>
                        )}
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {redditStaleOptimizations.map(o => (
                          <span key={o.id} className="text-xs px-2 py-1 bg-amber-500/10 text-amber-300 border border-amber-500/20 rounded-md">{o.question}</span>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Overall score */}
                  <div className="flex items-center gap-4">
                    <div className={`w-16 h-16 rounded-2xl border-2 flex items-center justify-center text-2xl font-bold ${scoreBadge(r.overall_score)}`}>
                      {r.overall_score}
                    </div>
                    <div>
                      <h2 className="text-white font-bold text-lg">{redditDomain}</h2>
                      <div className="flex items-center gap-2 text-dark-500 text-xs mt-0.5">
                        <span>{redditAnalysis.threads?.length || 0} threads analyzed</span>
                        <span>·</span>
                        <span>{fmtDate(redditAnalysis.generated_at)}</span>
                        {redditAnalysis.brand_context_used && (
                          <>
                            <span>·</span>
                            <span className="text-primary-400">Brand context used</span>
                          </>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* 4-Pillar Cards */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                    {[
                      { name: 'Presence', weight: '25%', score: r.presence?.score || 0, desc: 'Volume and breadth of mentions across subreddits' },
                      { name: 'Sentiment', weight: '25%', score: r.sentiment?.score || 0, desc: 'Community tone, recommendation frequency' },
                      { name: 'Competitive', weight: '25%', score: r.competitive?.score || 0, desc: 'Head-to-head positioning vs. competitors' },
                      { name: 'Training Signal', weight: '25%', score: r.training_signal?.score || 0, desc: 'Likelihood Reddit content influences LLM training' },
                    ].map(dim => (
                      <div key={dim.name} className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-5">
                        <div className="flex items-center justify-between mb-3">
                          <div>
                            <h4 className="text-white font-medium text-sm">{dim.name}</h4>
                            <span className="text-dark-500 text-xs">{dim.weight} weight</span>
                          </div>
                          <span className={`text-lg font-bold ${scoreTextColor(dim.score)}`}>{dim.score}</span>
                        </div>
                        <div className="h-1.5 bg-dark-800 rounded-full mb-3 overflow-hidden">
                          <div className={`h-full rounded-full ${barColor(dim.score)}`} style={{ width: `${dim.score}%` }} />
                        </div>
                        <p className="text-dark-500 text-xs leading-relaxed">{dim.desc}</p>
                      </div>
                    ))}
                  </div>

                  {/* Summary */}
                  {r.executive_summary && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <div className="flex items-center justify-between mb-3">
                        <h3 className="text-white font-semibold text-sm">Summary</h3>
                        <button onClick={() => setExpandedSections(prev => { const n = new Set(prev); n.has('reddit-summary') ? n.delete('reddit-summary') : n.add('reddit-summary'); return n })}
                          className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer">{expandedSections.has('reddit-summary') ? 'Collapse' : 'Expand'}</button>
                      </div>
                      <div className={`text-dark-300 text-sm leading-relaxed whitespace-pre-wrap ${!expandedSections.has('reddit-summary') ? 'line-clamp-3' : ''}`}>{r.executive_summary}</div>
                    </div>
                  )}

                  {/* Brand Recognition vs. Discovery callout */}
                  <div className="bg-orange-500/5 border border-orange-500/20 rounded-xl p-4 flex gap-3">
                    <svg className="w-4 h-4 text-orange-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 001.5-.189m-1.5.189a6.01 6.01 0 01-1.5-.189m3.75 7.478a12.06 12.06 0 01-4.5 0m3.75 2.383a14.406 14.406 0 01-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 10-7.517 0c.85.493 1.509 1.333 1.509 2.316V18" /></svg>
                    <p className="text-dark-400 text-xs leading-relaxed"><span className="text-orange-300 font-medium">Brand Recognition vs. Discovery:</span> Reddit naturally captures both. Direct brand mentions and sentiment reflect <span className="text-dark-300">recognition</span> — people who already know you are talking about you. Meanwhile, "best X for Y" recommendation threads are where people <span className="text-dark-300">discover</span> new solutions without prior brand awareness. Strong Reddit presence drives both brand authority and category discovery in LLM training data.</p>
                  </div>

                  {/* Sentiment breakdown */}
                  {r.sentiment?.sentiment && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-white font-semibold text-sm mb-4">What Reddit Thinks About You</h3>
                      <div className="grid grid-cols-3 gap-4 mb-4">
                        {[
                          { label: 'Positive', count: r.sentiment.sentiment.positive, color: 'text-emerald-400' },
                          { label: 'Neutral', count: r.sentiment.sentiment.neutral, color: 'text-dark-400' },
                          { label: 'Negative', count: r.sentiment.sentiment.negative, color: 'text-red-400' },
                        ].map(s => (
                          <div key={s.label} className="text-center">
                            <div className={`text-2xl font-bold ${s.color}`}>{s.count}</div>
                            <div className="text-dark-500 text-xs">{s.label}</div>
                          </div>
                        ))}
                      </div>
                      {r.sentiment.recommendation_rate > 0 && (
                        <div className="text-center mb-4 py-2 bg-emerald-500/10 border border-emerald-500/20 rounded-lg">
                          <span className="text-emerald-400 font-medium text-sm">{r.sentiment.recommendation_rate}% recommendation rate</span>
                        </div>
                      )}
                      {r.sentiment.top_praise?.length > 0 && (
                        <div className="mb-3">
                          <h4 className="text-dark-400 text-xs font-medium mb-1.5">Top Praise</h4>
                          <div className="flex flex-wrap gap-1.5">
                            {r.sentiment.top_praise.map((p, i) => (
                              <span key={i} className="text-xs px-2 py-1 bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 rounded-md">{p}</span>
                            ))}
                          </div>
                        </div>
                      )}
                      {r.sentiment.top_criticism?.length > 0 && (
                        <div>
                          <h4 className="text-dark-400 text-xs font-medium mb-1.5">Top Criticism</h4>
                          <div className="flex flex-wrap gap-1.5">
                            {r.sentiment.top_criticism.map((c, i) => (
                              <span key={i} className="text-xs px-2 py-1 bg-red-500/10 text-red-300 border border-red-500/20 rounded-md">{c}</span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}

                  {/* Share of Voice */}
                  {r.presence?.share_of_voice?.length > 0 && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-white font-semibold text-sm mb-3">Share of Voice</h3>
                      <div className="space-y-2">
                        {r.presence.share_of_voice.map((sov, i) => (
                          <div key={i} className="flex items-center gap-3">
                            <span className="text-white text-sm w-32 truncate">{sov.brand_name}</span>
                            <div className="flex-1 h-2 bg-dark-800 rounded-full overflow-hidden">
                              <div className={`h-full rounded-full ${i === 0 ? 'bg-orange-400' : 'bg-dark-600'}`} style={{ width: `${sov.percentage}%` }} />
                            </div>
                            <span className="text-dark-400 text-xs w-12 text-right">{sov.percentage}%</span>
                            <span className="text-dark-500 text-xs w-16 text-right">{sov.mention_count} mentions</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Competitive Positioning */}
                  {(r.competitive?.differentiators?.length > 0 || r.competitive?.competitor_strengths?.length > 0) && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-white font-semibold text-sm mb-4">Competitive Positioning</h3>
                      {r.competitive.win_rate > 0 && (
                        <div className="text-center mb-4 py-2 bg-primary-500/10 border border-primary-500/20 rounded-lg">
                          <span className="text-primary-400 font-medium text-sm">Win rate: {r.competitive.win_rate}% in head-to-head comparisons</span>
                          <span className="text-dark-500 text-xs ml-2">({r.competitive.comparison_threads} comparison threads)</span>
                        </div>
                      )}
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        {r.competitive.differentiators?.length > 0 && (
                          <div>
                            <h4 className="text-emerald-400 text-xs font-medium mb-2">Your Differentiators</h4>
                            <ul className="space-y-1">
                              {r.competitive.differentiators.map((d, i) => (
                                <li key={i} className="text-dark-300 text-xs flex items-start gap-1.5">
                                  <span className="text-emerald-400 mt-0.5">+</span>{d}
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                        {r.competitive.competitor_strengths?.length > 0 && (
                          <div>
                            <h4 className="text-red-400 text-xs font-medium mb-2">Competitor Advantages</h4>
                            <ul className="space-y-1">
                              {r.competitive.competitor_strengths.map((s, i) => (
                                <li key={i} className="text-dark-300 text-xs flex items-start gap-1.5">
                                  <span className="text-red-400 mt-0.5">-</span>{s}
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Training Signal */}
                  {r.training_signal && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-white font-semibold text-sm mb-3">LLM Training Signal</h3>
                      <div className="grid grid-cols-3 gap-4 mb-4">
                        <div className="text-center">
                          <div className="text-xl font-bold text-white">{r.training_signal.high_score_threads}</div>
                          <div className="text-dark-500 text-xs">High-score threads</div>
                        </div>
                        <div className="text-center">
                          <div className="text-xl font-bold text-white">{r.training_signal.deep_threads}</div>
                          <div className="text-dark-500 text-xs">Deep threads</div>
                        </div>
                        <div className="text-center">
                          <div className={`text-xl font-bold capitalize ${
                            r.training_signal.authority_tier === 'strong' ? 'text-emerald-400'
                            : r.training_signal.authority_tier === 'moderate' ? 'text-amber-400'
                            : 'text-red-400'
                          }`}>{r.training_signal.authority_tier}</div>
                          <div className="text-dark-500 text-xs">Authority tier</div>
                        </div>
                      </div>
                      {r.training_signal.evidence?.length > 0 && (
                        <ul className="space-y-1 mb-3">
                          {r.training_signal.evidence.map((e, i) => (
                            <li key={i} className="text-dark-400 text-xs flex items-start gap-1.5">
                              <span className="text-dark-600">•</span>{e}
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  )}

                  {/* Notable Mentions */}
                  {r.sentiment?.notable_mentions?.length > 0 && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-white font-semibold text-sm mb-3">Notable Mentions</h3>
                      <div className="space-y-3">
                        {r.sentiment.notable_mentions.map((m, i) => (
                          <div key={i} className="border border-dark-800 rounded-xl p-4">
                            <div className="flex items-center gap-2 mb-2">
                              <span className="text-xs px-1.5 py-0.5 bg-orange-500/15 text-orange-300 border border-orange-500/20 rounded">r/{m.subreddit}</span>
                              <span className={`text-xs px-1.5 py-0.5 rounded ${
                                m.sentiment === 'positive' ? 'bg-emerald-500/15 text-emerald-300'
                                : m.sentiment === 'negative' ? 'bg-red-500/15 text-red-300'
                                : 'bg-dark-700 text-dark-400'
                              }`}>{m.sentiment}</span>
                              {m.is_recommendation && <span className="text-xs px-1.5 py-0.5 bg-primary-500/15 text-primary-300 rounded">Recommends</span>}
                              <span className="text-dark-500 text-xs">{m.score} pts</span>
                            </div>
                            <p className="text-white text-sm mb-1">{m.title}</p>
                            {m.context && <p className="text-dark-400 text-xs italic">&ldquo;{m.context}&rdquo;</p>}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Recommendations */}
                  {r.recommendations?.length > 0 && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                      <h3 className="text-white font-semibold text-sm mb-3">Recommendations</h3>
                      <div className="space-y-3">
                        {r.recommendations.map((rec, i) => (
                          <div key={i} className="flex items-start gap-3 border border-dark-800 rounded-xl p-4">
                            <span className={`px-1.5 py-0.5 text-[10px] rounded font-medium mt-0.5 ${
                              rec.priority === 'high' ? 'bg-red-500/20 text-red-300'
                              : rec.priority === 'medium' ? 'bg-amber-500/20 text-amber-300'
                              : 'bg-dark-700 text-dark-400'
                            }`}>{rec.priority}</span>
                            <div className="flex-1">
                              <p className="text-white text-sm">{rec.action}</p>
                              <p className="text-dark-500 text-xs mt-0.5">{rec.expected_impact}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Confidence note */}
                  {r.confidence_note && (
                    <p className="text-dark-500 text-xs italic px-1">{r.confidence_note}</p>
                  )}
                </div>
              )
            })()}
          </div>
        )}

        {/* ─── Search Tab ──────────────────────────────────────── */}
        {activeTab === 'search' && (
          <div className="max-w-4xl mx-auto animate-fade-in">

            {/* No results yet — read-only placeholder in shared mode */}
            {!searchAnalysis && !searchAnalyzing && readOnly && (
              <div className="text-center py-16">
                <p className="text-dark-500 text-sm">No Search Visibility analysis available for this domain yet.</p>
              </div>
            )}
            {/* No results yet — show analyze prompt (authenticated only) */}
            {!searchAnalysis && !searchAnalyzing && !readOnly && (
              <>
                <div className="mb-6">
                  <div className="flex items-center gap-3 mb-1">
                    <h2 className="text-xl font-bold text-white">Search Visibility Analyzer</h2>
                  </div>
                  <p className="text-dark-400 text-sm">Analyze how search signals affect whether AI systems will discover, index, and cite your content.</p>
                </div>

                {/* Info cards */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-6">
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <svg className="w-4 h-4 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
                      <h3 className="text-white text-sm font-medium">AI Overview Readiness</h3>
                    </div>
                    <p className="text-dark-400 text-xs">76% of Google AI Overview citations come from top-10 organic pages. Structured data and content format matter.</p>
                  </div>
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <svg className="w-4 h-4 text-accent-emerald" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" /></svg>
                      <h3 className="text-white text-sm font-medium">Crawl Accessibility</h3>
                    </div>
                    <p className="text-dark-400 text-xs">GPTBot grew 305% YoY. AI crawlers need access to your content — robots.txt policy directly affects visibility.</p>
                  </div>
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <svg className="w-4 h-4 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18L9 11.25l4.306 4.307a11.95 11.95 0 015.814-5.519l2.74-1.22m0 0l-5.94-2.28m5.94 2.28l-2.28 5.941" /></svg>
                      <h3 className="text-white text-sm font-medium">Brand Momentum</h3>
                    </div>
                    <p className="text-dark-400 text-xs">Brand search volume has a 0.334 correlation with AI citation frequency. Web mentions are the strongest predictor.</p>
                  </div>
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-4">
                    <div className="flex items-center gap-2 mb-2">
                      <svg className="w-4 h-4 text-accent-purple" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                      <h3 className="text-white text-sm font-medium">Content Freshness</h3>
                    </div>
                    <p className="text-dark-400 text-xs">AI assistants cite content 25.7% newer than traditional search. 65% of AI bot hits target content less than 1 year old.</p>
                  </div>
                </div>

                {/* Auto-archive toggle */}
                <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-4 mb-6">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={searchAutoArchive}
                      onChange={e => setSearchAutoArchive(e.target.checked)}
                      className="w-4 h-4 rounded border-dark-600 bg-dark-800 text-primary-500 focus:ring-primary-500/30"
                    />
                    <div>
                      <span className="text-white text-sm font-medium">Auto-archive previous to-dos</span>
                      <p className="text-dark-500 text-xs mt-0.5">Archive existing Search to-dos before generating new ones</p>
                    </div>
                  </label>
                </div>

                {/* Analyze button */}
                <button
                  onClick={searchAnalyze}
                  disabled={!selectedDomain || readOnly}
                  className="w-full py-3.5 rounded-xl font-semibold text-white bg-gradient-to-r from-primary-600 to-primary-500 hover:from-primary-500 hover:to-primary-400 transition-all disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer text-sm"
                >
                  Analyze Search Visibility
                </button>

                {/* Failed analyses */}
                {failedAnalyses.filter(f => f.feed_type === 'search' && (!selectedDomain || domainKey(f.domain) === domainKey(selectedDomain))).length > 0 && (
                  <div className="mt-4 space-y-2">
                    {failedAnalyses.filter(f => f.feed_type === 'search' && (!selectedDomain || domainKey(f.domain) === domainKey(selectedDomain))).map(f => (
                      <div key={f.id} className="px-4 py-3 bg-dark-900/50 border border-amber-500/30 rounded-xl flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" />
                          <span className="text-amber-400 text-xs">
                            Failed ({errorCodeLabel(f.error_code)}) &middot; {fmtDate(f.failed_at)}
                          </span>
                        </div>
                        <button onClick={() => dismissFailedAnalysis(f.id)} className="text-dark-500 hover:text-dark-300 text-xs cursor-pointer">Dismiss</button>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}

            {/* Running state */}
            {searchAnalyzing && (
              <div className="bg-dark-900/50 border border-dark-800 rounded-2xl p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-white font-semibold flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full bg-primary-400 animate-pulse" />
                    Analyzing Search Visibility...
                  </h3>
                  <button onClick={searchAnalyzeStop} className="text-xs text-dark-400 hover:text-red-400 transition-colors cursor-pointer">Stop</button>
                </div>
                <div className="space-y-1 max-h-60 overflow-y-auto text-xs font-mono">
                  {searchMessages.map((msg, i) => (
                    <p key={i} className={`${msg.startsWith('Upstream API issue') ? 'text-amber-400' : msg.startsWith('Error') ? 'text-red-400' : 'text-dark-400'}`}>{msg}</p>
                  ))}
                  <div ref={searchMessagesEndRef} />
                </div>
              </div>
            )}

            {/* Results view */}
            {searchAnalysis?.result && !searchAnalyzing && (() => {
              const r = searchAnalysis.result!
              const scoreColor = (s: number) => s >= 70 ? 'text-accent-emerald' : s >= 40 ? 'text-amber-400' : 'text-red-400'
              const scoreBg = (s: number) => s >= 70 ? 'bg-accent-emerald/20 border-accent-emerald/30' : s >= 40 ? 'bg-amber-500/20 border-amber-500/30' : 'bg-red-500/20 border-red-500/30'
              const barColor = (s: number) => s >= 70 ? 'bg-accent-emerald' : s >= 40 ? 'bg-amber-400' : 'bg-red-400'

              return (
                <div className="space-y-6">
                  {/* Header with overall score */}
                  <div className="flex items-center justify-between">
                    <div>
                      <h2 className="text-xl font-bold text-white">Search Visibility</h2>
                      <p className="text-dark-400 text-sm mt-1">
                        Analyzed {new Date(searchAnalysis.generated_at).toLocaleDateString()} · {searchAnalysis.model}
                        {searchAnalysis.brand_context_used && <span className="ml-2 text-xs text-primary-400">+ Brand Intelligence</span>}
                      </p>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className={`text-3xl font-bold ${scoreColor(r.overall_score)}`}>{r.overall_score}</div>
                      <div className="text-dark-500 text-xs">/100</div>
                      {!readOnly && (
                        <div className="flex items-center gap-1 ml-3">
                          <button onClick={searchAnalyze} className="px-3 py-1.5 text-xs font-medium text-primary-400 border border-primary-500/30 rounded-lg hover:bg-primary-500/10 transition-colors cursor-pointer" title="Re-analyze">
                            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182" /></svg>
                          </button>
                          <button onClick={() => setConfirmDeleteSearchAnalysis(true)} className="px-3 py-1.5 text-xs font-medium text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors cursor-pointer" title="Delete">
                            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
                          </button>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Stale: new optimization questions since this analysis */}
                  {searchStaleOptimizations.length > 0 && (
                    <div className="bg-amber-500/10 border border-amber-500/20 rounded-xl p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-2">
                          <svg className="w-4 h-4 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
                          <span className="text-amber-400 text-sm font-medium">
                            {searchStaleOptimizations.length} new optimization {searchStaleOptimizations.length === 1 ? 'question' : 'questions'} since this search analysis
                          </span>
                        </div>
                        {!readOnly && (
                          <button onClick={searchAnalyze} className="text-xs px-3 py-1.5 bg-amber-500/10 border border-amber-500/30 text-amber-400 rounded-lg hover:bg-amber-500/20 transition-all cursor-pointer whitespace-nowrap shrink-0">
                            Re-run Analysis
                          </button>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Executive Summary */}
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                    <div className="flex items-center justify-between mb-3">
                      <h3 className="text-white font-medium text-sm">Executive Summary</h3>
                      <button onClick={() => setExpandedSections(prev => { const n = new Set(prev); n.has('search-summary') ? n.delete('search-summary') : n.add('search-summary'); return n })}
                        className="text-xs text-primary-400 hover:text-primary-300 cursor-pointer">{expandedSections.has('search-summary') ? 'Collapse' : 'Expand'}</button>
                    </div>
                    <div className={`text-dark-300 text-sm leading-relaxed whitespace-pre-line ${!expandedSections.has('search-summary') ? 'line-clamp-3' : ''}`}>{r.executive_summary}</div>
                  </div>

                  {/* Brand Recognition vs Discovery callout */}
                  {r.category_discovery && (
                    <div className="bg-cyan-500/5 border border-cyan-500/20 rounded-xl px-5 py-3">
                      <p className="text-dark-400 text-xs leading-relaxed">
                        <span className="text-cyan-400 font-medium">Brand Recognition vs. Discovery:</span>{' '}
                        This analysis measures both how visible you are when people search for your brand by name (Brand Momentum)
                        and how discoverable you are when people search your category without knowing you exist (Category Discovery).
                      </p>
                    </div>
                  )}

                  {/* Pillar Scores */}
                  <div className={`grid grid-cols-2 ${r.category_discovery ? 'sm:grid-cols-5' : 'sm:grid-cols-4'} gap-3`}>
                    {[
                      { label: 'AI Overview Readiness', score: r.aio_readiness.score, weight: r.category_discovery ? '25%' : '30%' },
                      { label: 'Crawl Accessibility', score: r.crawl_accessibility.score, weight: r.category_discovery ? '15%' : '20%' },
                      { label: 'Brand Momentum', score: r.brand_momentum.score, weight: r.category_discovery ? '20%' : '25%' },
                      { label: 'Content Freshness', score: r.content_freshness.score, weight: r.category_discovery ? '20%' : '25%' },
                      ...(r.category_discovery ? [{ label: 'Category Discovery', score: r.category_discovery.score, weight: '20%' }] : []),
                    ].map((p, i) => (
                      <div key={i} className={`border rounded-xl p-4 text-center ${scoreBg(p.score)}`}>
                        <div className={`text-2xl font-bold ${scoreColor(p.score)}`}>{p.score}</div>
                        <div className="text-white text-xs font-medium mt-1">{p.label}</div>
                        <div className="text-dark-500 text-[10px] mt-0.5">{p.weight} weight</div>
                      </div>
                    ))}
                  </div>

                  {/* AIO Readiness Detail */}
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-white font-medium text-sm flex items-center gap-2">
                        <svg className="w-4 h-4 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
                        AI Overview Readiness
                      </h3>
                      <span className={`text-lg font-bold ${scoreColor(r.aio_readiness.score)}`}>{r.aio_readiness.score}/100</span>
                    </div>
                    <div className="grid grid-cols-2 gap-3 mb-4">
                      {[
                        { label: 'Organic Presence', value: r.aio_readiness.organic_presence },
                        { label: 'Structured Data', value: r.aio_readiness.structured_data },
                        { label: 'Content Format', value: r.aio_readiness.content_format },
                        { label: 'Answer Prominence', value: r.aio_readiness.answer_prominence },
                      ].map((m, i) => (
                        <div key={i}>
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-dark-400 text-xs">{m.label}</span>
                            <span className={`text-xs font-medium ${scoreColor(m.value)}`}>{m.value}</span>
                          </div>
                          <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                            <div className={`h-full rounded-full ${barColor(m.value)}`} style={{ width: `${m.value}%` }} />
                          </div>
                        </div>
                      ))}
                    </div>
                    <div className="space-y-1">
                      {r.aio_readiness.evidence.map((e, i) => (
                        <p key={i} className="text-dark-400 text-xs flex items-start gap-2"><span className="text-primary-400 mt-0.5">•</span>{e}</p>
                      ))}
                    </div>
                  </div>

                  {/* Crawl Accessibility Detail */}
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-white font-medium text-sm flex items-center gap-2">
                        <svg className="w-4 h-4 text-accent-emerald" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" /></svg>
                        Crawl Accessibility
                      </h3>
                      <span className={`text-lg font-bold ${scoreColor(r.crawl_accessibility.score)}`}>{r.crawl_accessibility.score}/100</span>
                    </div>
                    {/* robots.txt policy */}
                    <div className="bg-dark-800/50 rounded-lg p-3 mb-4">
                      <p className="text-dark-400 text-xs font-medium mb-1">robots.txt Policy</p>
                      <p className="text-dark-300 text-sm">{r.crawl_accessibility.robots_txt_policy}</p>
                    </div>
                    {/* Crawler details */}
                    {r.crawl_accessibility.crawler_details?.length > 0 && (
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 mb-4">
                        {r.crawl_accessibility.crawler_details.map((c, i) => (
                          <div key={i} className="flex items-center gap-2 bg-dark-800/30 rounded-lg px-3 py-2">
                            <span className={`w-2 h-2 rounded-full ${c.allowed ? 'bg-accent-emerald' : 'bg-red-400'}`} />
                            <span className="text-white text-xs font-medium">{c.name}</span>
                            <span className={`text-xs ${c.allowed ? 'text-accent-emerald' : 'text-red-400'}`}>{c.allowed ? 'Allowed' : 'Blocked'}</span>
                            {c.notes && <span className="text-dark-500 text-[10px] ml-auto truncate max-w-[120px]" title={c.notes}>{c.notes}</span>}
                          </div>
                        ))}
                      </div>
                    )}
                    <div className="grid grid-cols-3 gap-3 mb-4">
                      {[
                        { label: 'AI Bot Access', value: r.crawl_accessibility.ai_bot_access },
                        { label: 'Sitemap Quality', value: r.crawl_accessibility.sitemap_quality },
                        { label: 'Render Access', value: r.crawl_accessibility.render_accessibility },
                      ].map((m, i) => (
                        <div key={i}>
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-dark-400 text-xs">{m.label}</span>
                            <span className={`text-xs font-medium ${scoreColor(m.value)}`}>{m.value}</span>
                          </div>
                          <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                            <div className={`h-full rounded-full ${barColor(m.value)}`} style={{ width: `${m.value}%` }} />
                          </div>
                        </div>
                      ))}
                    </div>
                    <div className="space-y-1">
                      {r.crawl_accessibility.evidence.map((e, i) => (
                        <p key={i} className="text-dark-400 text-xs flex items-start gap-2"><span className="text-accent-emerald mt-0.5">•</span>{e}</p>
                      ))}
                    </div>
                  </div>

                  {/* Brand Momentum Detail */}
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-white font-medium text-sm flex items-center gap-2">
                        <svg className="w-4 h-4 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18L9 11.25l4.306 4.307a11.95 11.95 0 015.814-5.519l2.74-1.22m0 0l-5.94-2.28m5.94 2.28l-2.28 5.941" /></svg>
                        Brand Search Momentum
                      </h3>
                      <span className={`text-lg font-bold ${scoreColor(r.brand_momentum.score)}`}>{r.brand_momentum.score}/100</span>
                    </div>
                    <div className="grid grid-cols-2 gap-4 mb-4">
                      <div className="bg-dark-800/50 rounded-lg p-3">
                        <p className="text-dark-400 text-xs font-medium mb-1">Search Trend</p>
                        <p className={`text-sm font-medium ${r.brand_momentum.brand_search_trend === 'growing' ? 'text-accent-emerald' : r.brand_momentum.brand_search_trend === 'declining' ? 'text-red-400' : 'text-amber-400'}`}>
                          {r.brand_momentum.brand_search_trend === 'growing' ? '↑ Growing' : r.brand_momentum.brand_search_trend === 'declining' ? '↓ Declining' : '→ Stable'}
                        </p>
                      </div>
                      <div className="bg-dark-800/50 rounded-lg p-3">
                        <p className="text-dark-400 text-xs font-medium mb-1">Entity Recognition</p>
                        <div className="flex items-center gap-2">
                          <div className="flex-1 h-1.5 bg-dark-700 rounded-full overflow-hidden">
                            <div className={`h-full rounded-full ${barColor(r.brand_momentum.entity_recognition)}`} style={{ width: `${r.brand_momentum.entity_recognition}%` }} />
                          </div>
                          <span className={`text-xs font-medium ${scoreColor(r.brand_momentum.entity_recognition)}`}>{r.brand_momentum.entity_recognition}</span>
                        </div>
                      </div>
                    </div>
                    {r.brand_momentum.competitor_compare && (
                      <div className="bg-dark-800/50 rounded-lg p-3 mb-4">
                        <p className="text-dark-400 text-xs font-medium mb-1">vs. Competitors</p>
                        <p className="text-dark-300 text-xs leading-relaxed">{r.brand_momentum.competitor_compare}</p>
                      </div>
                    )}
                    <div className="space-y-1">
                      {r.brand_momentum.evidence.map((e, i) => (
                        <p key={i} className="text-dark-400 text-xs flex items-start gap-2"><span className="text-amber-400 mt-0.5">•</span>{e}</p>
                      ))}
                    </div>
                  </div>

                  {/* Content Freshness Detail */}
                  <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-white font-medium text-sm flex items-center gap-2">
                        <svg className="w-4 h-4 text-accent-purple" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                        Content Freshness
                      </h3>
                      <span className={`text-lg font-bold ${scoreColor(r.content_freshness.score)}`}>{r.content_freshness.score}/100</span>
                    </div>
                    <div className="grid grid-cols-2 gap-4 mb-4">
                      <div className="bg-dark-800/50 rounded-lg p-3">
                        <p className="text-dark-400 text-xs font-medium mb-1">Content Age</p>
                        <p className="text-dark-300 text-sm">{r.content_freshness.average_content_age}</p>
                      </div>
                      <div className="bg-dark-800/50 rounded-lg p-3">
                        <p className="text-dark-400 text-xs font-medium mb-1">Update Frequency</p>
                        <p className={`text-sm font-medium ${r.content_freshness.update_frequency === 'frequent' ? 'text-accent-emerald' : r.content_freshness.update_frequency === 'stale' ? 'text-red-400' : 'text-amber-400'}`}>
                          {r.content_freshness.update_frequency.charAt(0).toUpperCase() + r.content_freshness.update_frequency.slice(1)}
                        </p>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-3 mb-4">
                      {[
                        { label: 'Freshness Signals', value: r.content_freshness.freshness_signals },
                        { label: 'Content Decay Risk', value: 100 - r.content_freshness.content_decay_risk },
                      ].map((m, i) => (
                        <div key={i}>
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-dark-400 text-xs">{m.label}</span>
                            <span className={`text-xs font-medium ${scoreColor(m.value)}`}>{m.value}</span>
                          </div>
                          <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                            <div className={`h-full rounded-full ${barColor(m.value)}`} style={{ width: `${m.value}%` }} />
                          </div>
                        </div>
                      ))}
                    </div>
                    <div className="space-y-1">
                      {r.content_freshness.evidence.map((e, i) => (
                        <p key={i} className="text-dark-400 text-xs flex items-start gap-2"><span className="text-accent-purple mt-0.5">•</span>{e}</p>
                      ))}
                    </div>
                  </div>

                  {/* Category Discovery Detail */}
                  {r.category_discovery && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                      <div className="flex items-center justify-between mb-4">
                        <h3 className="text-white font-medium text-sm flex items-center gap-2">
                          <svg className="w-4 h-4 text-teal-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" /></svg>
                          Category Discovery
                        </h3>
                        <span className={`text-lg font-bold ${scoreColor(r.category_discovery.score)}`}>{r.category_discovery.score}/100</span>
                      </div>
                      <div className="grid grid-cols-2 gap-3 mb-4">
                        {[
                          { label: 'Category Visibility', value: r.category_discovery.category_visibility },
                          { label: 'Intent Coverage', value: r.category_discovery.intent_coverage },
                          { label: 'Competitor Gap', value: r.category_discovery.competitor_gap },
                          { label: 'Discovery Potential', value: r.category_discovery.discovery_potential },
                        ].map((m, i) => (
                          <div key={i}>
                            <div className="flex items-center justify-between mb-1">
                              <span className="text-dark-400 text-xs">{m.label}</span>
                              <span className={`text-xs font-medium ${scoreColor(m.value)}`}>{m.value}</span>
                            </div>
                            <div className="h-1.5 bg-dark-800 rounded-full overflow-hidden">
                              <div className={`h-full rounded-full ${barColor(m.value)}`} style={{ width: `${m.value}%` }} />
                            </div>
                          </div>
                        ))}
                      </div>
                      <div className="space-y-1">
                        {r.category_discovery.evidence.map((e, i) => (
                          <p key={i} className="text-dark-400 text-xs flex items-start gap-2"><span className="text-teal-400 mt-0.5">•</span>{e}</p>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Recommendations */}
                  {r.recommendations?.length > 0 && (
                    <div className="bg-dark-900/50 border border-dark-800 rounded-xl p-5">
                      <h3 className="text-white font-medium text-sm mb-4">Recommendations</h3>
                      <div className="space-y-3">
                        {r.recommendations.map((rec, i) => (
                          <div key={i} className="flex items-start gap-3">
                            <span className={`shrink-0 px-1.5 py-0.5 text-[10px] font-bold rounded uppercase ${rec.priority === 'high' ? 'bg-red-500/20 text-red-400' : rec.priority === 'medium' ? 'bg-amber-500/20 text-amber-400' : 'bg-dark-700 text-dark-400'}`}>{rec.priority}</span>
                            <div className="flex-1 min-w-0">
                              <p className="text-white text-sm">{rec.action}</p>
                              <p className="text-dark-400 text-xs mt-0.5">{rec.expected_impact}</p>
                              <p className="text-dark-500 text-[10px] mt-0.5">{rec.dimension}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Confidence note */}
                  {r.confidence_note && (
                    <p className="text-dark-500 text-xs italic px-1">{r.confidence_note}</p>
                  )}
                </div>
              )
            })()}
          </div>
        )}
        {activeTab === 'test' && (
          <div className="max-w-5xl mx-auto animate-fade-in">

            {/* Cross-report insights */}
            {crossInsights.filter(i => i.tab === 'test' && !dismissedInsights.has(i.message)).map(insight => (
              <div key={insight.message} className="flex items-center justify-between bg-primary-500/5 border border-primary-500/20 rounded-xl px-4 py-3 mb-4">
                <span className="text-dark-300 text-xs flex-1 mr-3">{insight.message}</span>
                <div className="flex items-center gap-2 shrink-0">
                  <button onClick={() => setActiveTab(insight.targetTab as typeof activeTab)} className="text-xs px-3 py-1 bg-primary-500/10 border border-primary-500/20 text-primary-400 rounded-lg hover:bg-primary-500/20 transition-all cursor-pointer">{insight.cta}</button>
                  <button onClick={() => setDismissedInsights(prev => new Set([...prev, insight.message]))} className="text-dark-600 hover:text-dark-400 cursor-pointer"><svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg></button>
                </div>
              </div>
            ))}

            {/* Results view */}
            {testView === 'results' && testResults && (() => {
              const r = testResults
              const scoreColor = (s: number) => s >= 70 ? 'text-emerald-400' : s >= 40 ? 'text-amber-400' : 'text-red-400'
              const scoreBg = (s: number) => s >= 70 ? 'bg-emerald-500/10 border-emerald-500/30' : s >= 40 ? 'bg-amber-500/10 border-amber-500/30' : 'bg-red-500/10 border-red-500/30'
              const sentimentColor = (s: string) => s === 'positive' ? 'text-emerald-400' : s === 'neutral' ? 'text-dark-400' : s === 'negative' ? 'text-red-400' : 'text-dark-600'
              const sentimentIcon = (s: string) => s === 'positive' ? '+' : s === 'neutral' ? '~' : s === 'negative' ? '-' : '?'

              return (
                <div className="space-y-6">
                  {/* Back button row */}
                  <div className="flex items-center justify-between">
                    {!readOnly ? (
                      <button
                        onClick={() => { setTestView('input'); setTestResults(null) }}
                        className="text-dark-400 hover:text-white transition-colors cursor-pointer text-sm flex items-center gap-1"
                      >
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
                        Re-run or Change Settings
                      </button>
                    ) : <div />}
                    <div className="flex items-center gap-3">
                      {saasEnabled && user && (user.role === 'owner' || user.role === 'admin') && selectedDomain && (
                        <button onClick={() => { setDomainShareState(null); setShareModalDomain(selectedDomain); fetchDomainShare(selectedDomain) }}
                          className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:bg-dark-700 hover:text-white transition-all cursor-pointer flex items-center gap-1.5"
                          title="Share this domain"
                        >
                          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 100 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186l9.566-5.314m-9.566 7.5l9.566 5.314m0 0a2.25 2.25 0 103.935 2.186 2.25 2.25 0 00-3.935-2.186zm0-12.814a2.25 2.25 0 103.933-2.185 2.25 2.25 0 00-3.933 2.185z" /></svg>
                          Share
                        </button>
                      )}
                      {!readOnly && (
                        <>
                          <button onClick={() => setShowCompetitorModal(true)} className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-amber-400 rounded-lg hover:bg-amber-900/30 hover:border-amber-800 transition-all cursor-pointer">Test Competitor</button>
                          <button onClick={() => setConfirmDeleteTest(true)} className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-red-400 rounded-lg hover:bg-red-900/30 hover:border-red-800 transition-all cursor-pointer">Delete</button>
                          <button onClick={() => { setTestView('input'); setTestResults(null) }} className="text-xs px-3 py-1.5 bg-dark-800 border border-dark-700 text-primary-400 rounded-lg hover:bg-dark-700 hover:text-primary-300 transition-all cursor-pointer">Re-run Test</button>
                        </>
                      )}
                    </div>
                  </div>

                  {/* Overall score header */}
                  <div className="flex items-center gap-4">
                    <div className={`w-16 h-16 rounded-xl border flex items-center justify-center ${scoreBg(r.overall_score)}`}>
                      <span className={`text-2xl font-bold ${scoreColor(r.overall_score)}`}>{r.overall_score}</span>
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center gap-3">
                        <h2 className="text-xl font-bold text-white">LLM Brand Test — {r.brand_name}</h2>
                        {r.run_number > 0 && <span className="text-xs text-dark-500 bg-dark-800 px-2 py-0.5 rounded-full">Run #{r.run_number}</span>}
                      </div>
                      <p className="text-dark-400 text-sm">{r.provider_summaries.length} provider{r.provider_summaries.length !== 1 ? 's' : ''} tested with {r.queries.length} queries &middot; {fmtDate(r.generated_at)}</p>
                    </div>
                    {/* Trend sparkline */}
                    {testHistory.length > 1 && (() => {
                      const points = [...testHistory].reverse().slice(-8)
                      const maxScore = Math.max(...points.map(p => p.overall_score), 1)
                      const prevScore = points.length >= 2 ? points[points.length - 2].overall_score : null
                      const delta = prevScore !== null ? r.overall_score - prevScore : null
                      return (
                        <div className="flex items-center gap-3">
                          <div className="flex items-end gap-0.5 h-8">
                            {points.map((p, i) => (
                              <div
                                key={p.id || i}
                                className={`w-2 rounded-t transition-all ${i === points.length - 1 ? 'bg-primary-400' : 'bg-dark-600'}`}
                                style={{ height: `${Math.max(4, (p.overall_score / maxScore) * 32)}px` }}
                                title={`Run #${p.run_number}: ${p.overall_score}`}
                              />
                            ))}
                          </div>
                          {delta !== null && (
                            <span className={`text-xs font-medium ${delta > 0 ? 'text-emerald-400' : delta < 0 ? 'text-red-400' : 'text-dark-500'}`}>
                              {delta > 0 ? '+' : ''}{delta}
                            </span>
                          )}
                        </div>
                      )
                    })()}
                  </div>

                  {/* Compare with previous run */}
                  {testHistory.length > 1 && (
                    <div className="flex items-center gap-3">
                      {!testCompareRun ? (
                        <div className="flex items-center gap-2">
                          <span className="text-dark-500 text-xs">Compare with:</span>
                          <select
                            onChange={e => {
                              const run = testHistory.find(h => h.id === e.target.value)
                              setTestCompareRun(run || null)
                            }}
                            value=""
                            className="text-xs bg-dark-800 border border-dark-700 rounded-md px-2 py-1 text-dark-300 cursor-pointer"
                          >
                            <option value="">Select a previous run</option>
                            {testHistory.filter(h => h.id !== r.id).map(h => (
                              <option key={h.id} value={h.id}>Run #{h.run_number} — Score {h.overall_score} ({fmtDate(h.generated_at)})</option>
                            ))}
                          </select>
                        </div>
                      ) : (
                        <div className="flex items-center gap-3 bg-dark-800/50 border border-dark-700 rounded-lg px-3 py-2">
                          <span className="text-dark-400 text-xs">Comparing with Run #{testCompareRun.run_number}</span>
                          <span className={`text-xs font-medium ${r.overall_score - testCompareRun.overall_score > 0 ? 'text-emerald-400' : r.overall_score - testCompareRun.overall_score < 0 ? 'text-red-400' : 'text-dark-500'}`}>
                            {r.overall_score - testCompareRun.overall_score > 0 ? '+' : ''}{r.overall_score - testCompareRun.overall_score} overall
                          </span>
                          <button onClick={() => setTestCompareRun(null)} className="text-dark-500 hover:text-white text-xs cursor-pointer ml-1">Clear</button>
                        </div>
                      )}
                    </div>
                  )}

                  {/* Competitor overlay toggle */}
                  {competitorTests.length > 0 && (
                    <div className="flex items-center gap-3 flex-wrap">
                      <label className="flex items-center gap-2 cursor-pointer">
                        <input type="checkbox" checked={showCompetitorOverlay} onChange={e => setShowCompetitorOverlay(e.target.checked)}
                          className="w-3.5 h-3.5 rounded border-dark-600 bg-dark-800 text-amber-500 focus:ring-amber-500/30 cursor-pointer" />
                        <span className="text-dark-400 text-xs">Show competitor comparison</span>
                      </label>
                      <div className="flex gap-1.5">
                        {competitorTests.map(ct => (
                          <span key={ct.domain} className="text-[10px] px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20">{ct.brand_name || ct.domain}</span>
                        ))}
                      </div>
                      {showCompetitorOverlay && (
                        <button
                          onClick={() => {
                            const lines = [`AI Brand Comparison — ${r.brand_name}`, '']
                            r.provider_summaries.forEach(ps => {
                              const myLine = `${ps.provider_name}: ${r.brand_name} ${ps.overall_score}/100`
                              const compLines = competitorTests.map(ct => {
                                const cps = ct.provider_summaries.find(s => s.provider_id === ps.provider_id)
                                return cps ? `  vs ${ct.brand_name || ct.domain}: ${cps.overall_score}/100 (${ps.overall_score - cps.overall_score > 0 ? '+' : ''}${ps.overall_score - cps.overall_score})` : ''
                              }).filter(Boolean)
                              lines.push(myLine, ...compLines, '')
                            })
                            lines.push('Powered by LLM Optimizer — llmopt.metavert.io')
                            navigator.clipboard.writeText(lines.join('\n'))
                          }}
                          className="text-[10px] px-2.5 py-1 bg-dark-800 border border-dark-700 text-dark-400 rounded-lg hover:text-white hover:border-dark-600 transition-all cursor-pointer"
                        >
                          Copy Comparison
                        </button>
                      )}
                    </div>
                  )}

                  {/* Brand Recognition vs. Discovery callout */}
                  <div className="bg-amber-500/5 border border-amber-500/20 rounded-xl p-4 flex gap-3">
                    <svg className="w-4 h-4 text-amber-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 001.5-.189m-1.5.189a6.01 6.01 0 01-1.5-.189m3.75 7.478a12.06 12.06 0 01-4.5 0m3.75 2.383a14.406 14.406 0 01-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 10-7.517 0c.85.493 1.509 1.333 1.509 2.316V18" /></svg>
                    <p className="text-dark-400 text-xs leading-relaxed"><span className="text-amber-300 font-medium">Brand Recognition vs. Discovery:</span> This test directly measures both. <span className="text-dark-300">Brand</span> queries test recognition — do LLMs know who you are and represent you accurately? <span className="text-dark-300">Category</span> and <span className="text-dark-300">discovery</span> queries test whether LLMs recommend you to people who don't yet know your brand, searching only by category or use case.</p>
                  </div>

                  {/* Provider summary cards */}
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                    {r.provider_summaries.map(ps => {
                      const compScores = showCompetitorOverlay ? competitorTests.map(ct => {
                        const cps = ct.provider_summaries.find(s => s.provider_id === ps.provider_id)
                        return cps ? { name: ct.brand_name || ct.domain, score: cps.overall_score, mention: cps.mention_rate } : null
                      }).filter(Boolean) as { name: string; score: number; mention: number }[] : []
                      const prevPs = testCompareRun?.provider_summaries.find(s => s.provider_id === ps.provider_id)
                      return (
                        <div key={ps.provider_id} className="bg-dark-800/50 border border-dark-700 rounded-xl p-4">
                          <div className="flex items-center justify-between mb-3">
                            <div>
                              <h3 className="text-white font-semibold text-sm">{ps.provider_name}</h3>
                              <p className="text-dark-500 text-xs">{ps.model}</p>
                            </div>
                            <div className="flex items-center gap-2">
                              {prevPs && (() => {
                                const d = ps.overall_score - prevPs.overall_score
                                return d !== 0 ? <span className={`text-xs font-medium ${d > 0 ? 'text-emerald-400' : 'text-red-400'}`}>{d > 0 ? '+' : ''}{d}</span> : null
                              })()}
                            </div>
                            <div className={`w-12 h-12 rounded-lg border flex items-center justify-center ${scoreBg(ps.overall_score)}`}>
                              <span className={`text-lg font-bold ${scoreColor(ps.overall_score)}`}>{ps.overall_score}</span>
                            </div>
                          </div>
                          <div className="space-y-2 text-xs">
                            <div className="flex justify-between"><span className="text-dark-400">Mention Rate</span><span className="text-white font-medium">{ps.mention_rate}%</span></div>
                            <div className="flex justify-between"><span className="text-dark-400">Recommend Rate</span><span className="text-white font-medium">{ps.recommend_rate}%</span></div>
                            <div className="flex justify-between"><span className="text-dark-400">Accuracy</span><span className="text-white font-medium">{ps.accuracy_rate}%</span></div>
                            <div className="flex justify-between"><span className="text-dark-400">Sentiment</span><span className="text-white font-medium">{ps.sentiment_score}/100</span></div>
                          </div>
                          {compScores.length > 0 && (
                            <div className="mt-3 pt-3 border-t border-dark-700 space-y-1.5">
                              {compScores.map(cs => {
                                const delta = ps.overall_score - cs.score
                                return (
                                  <div key={cs.name} className="flex items-center justify-between text-xs">
                                    <span className="text-amber-400/70 truncate max-w-[60%]">vs {cs.name}</span>
                                    <div className="flex items-center gap-2">
                                      <span className="text-dark-400">{cs.score}</span>
                                      <span className={`font-medium ${delta > 0 ? 'text-emerald-400' : delta < 0 ? 'text-red-400' : 'text-dark-500'}`}>
                                        {delta > 0 ? '+' : ''}{delta}
                                      </span>
                                    </div>
                                  </div>
                                )
                              })}
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>

                  {/* Query results comparison */}
                  <div className="space-y-4">
                    <h3 className="text-white font-semibold text-lg">Query Results</h3>
                    {r.results.map((qr, qi) => {
                      const typeLabel: Record<string, string> = { brand: 'Brand', category: 'Category', comparison: 'Comparison', discovery: 'Discovery', custom: 'Custom' }
                      return (
                        <div key={qi} className="bg-dark-800/30 border border-dark-700 rounded-xl overflow-hidden">
                          <div className="px-4 py-3 border-b border-dark-700 flex items-center gap-3">
                            <span className="text-xs px-2 py-0.5 rounded bg-dark-700 text-dark-300 font-medium">{typeLabel[qr.query.type] || qr.query.type}</span>
                            <span className="text-white text-sm font-medium">{qr.query.query}</span>
                          </div>
                          <div className="divide-y divide-dark-700/50">
                            {qr.provider_results.map((pr, pi) => {
                              const cellKey = `${qi}-${pi}`
                              const expanded = testExpandedCells.has(cellKey)
                              return (
                                <div key={pi} className="px-4 py-3">
                                  <div className="flex items-center justify-between mb-1">
                                    <div className="flex items-center gap-3">
                                      <span className="text-dark-400 text-xs font-medium w-20">{pr.provider_name}</span>
                                      <div className="flex items-center gap-2 text-xs">
                                        <span className={pr.mentioned ? 'text-emerald-400' : 'text-dark-600'} title={pr.mentioned ? 'Brand mentioned' : 'Not mentioned'}>
                                          {pr.mentioned ? '✓ Mentioned' : '✗ Not mentioned'}
                                        </span>
                                        {pr.recommended && <span className="text-amber-400" title="Brand recommended">★ Recommended</span>}
                                        <span className={sentimentColor(pr.sentiment)} title={`Sentiment: ${pr.sentiment}`}>{sentimentIcon(pr.sentiment)} {pr.sentiment}</span>
                                      </div>
                                    </div>
                                    <div className="flex items-center gap-2">
                                      <span className={`text-sm font-bold ${scoreColor(pr.score)}`}>{pr.score}</span>
                                      <button
                                        onClick={() => setTestExpandedCells(prev => {
                                          const next = new Set(prev)
                                          if (next.has(cellKey)) next.delete(cellKey)
                                          else next.add(cellKey)
                                          return next
                                        })}
                                        className="text-dark-500 hover:text-white transition-colors cursor-pointer"
                                      >
                                        <svg className={`w-4 h-4 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
                                      </button>
                                    </div>
                                  </div>
                                  {!expanded && (
                                    <p className="text-dark-400 text-xs line-clamp-2 cursor-pointer" onClick={() => setTestExpandedCells(prev => { const next = new Set(prev); next.add(cellKey); return next })}>{pr.response}</p>
                                  )}
                                  {expanded && (
                                    <div className="mt-2 p-3 bg-dark-900/50 rounded-lg text-dark-300 text-xs whitespace-pre-wrap max-h-96 overflow-y-auto">{pr.response}</div>
                                  )}
                                </div>
                              )
                            })}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>
              )
            })()}

            {/* Analyzing state */}
            {testAnalyzing && (
              <div className="space-y-4">
                <div className="flex items-center gap-3 mb-2">
                  <div className="w-5 h-5 border-2 border-primary-400 border-t-transparent rounded-full animate-spin" />
                  <h2 className="text-lg font-semibold text-white">Testing LLMs...</h2>
                  <button onClick={stopLLMTest} className="text-xs px-3 py-1 bg-dark-800 border border-dark-700 text-dark-300 rounded-lg hover:text-white transition-colors cursor-pointer ml-auto">Stop</button>
                </div>
                <div className="bg-dark-800/50 border border-dark-700 rounded-xl p-4 max-h-64 overflow-y-auto">
                  {testMessages.map((m, i) => (
                    <p key={i} className="text-dark-400 text-xs py-0.5">{m}</p>
                  ))}
                  <div ref={testMessagesEndRef} />
                </div>
              </div>
            )}

            {/* Setup view — read-only placeholder when no results in shared mode */}
            {testView === 'input' && !testAnalyzing && !testResults && readOnly && (
              <div className="text-center py-16">
                <p className="text-dark-500 text-sm">No LLM Brand Test results available for this domain yet.</p>
              </div>
            )}
            {testView === 'input' && !testAnalyzing && !testResults && !readOnly && (
              <div className="space-y-6">
                <div>
                  <h2 className="text-xl font-bold text-white mb-1">LLM Brand Test</h2>
                  <p className="text-dark-400 text-sm">Ask ChatGPT, Claude, Gemini, and Grok about your brand and see their actual answers. Find out if AI recommends you — or your competitors.</p>
                </div>

                {testError && (
                  <div className="bg-red-900/20 border border-red-800/30 rounded-xl p-3 text-red-400 text-sm">{testError}</div>
                )}

                {/* Provider selection */}
                <div className="bg-dark-800/50 border border-dark-700 rounded-xl p-4">
                  <h3 className="text-white font-semibold text-sm mb-3">Select AI Providers to Test</h3>
                  {testAvailableProviders.length === 0 ? (
                    <p className="text-dark-500 text-sm">No API keys configured. <button onClick={() => { window.location.href = '/last/settings' }} className="text-primary-400 hover:text-primary-300 cursor-pointer underline">Set up API keys</button> to get started.</p>
                  ) : (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                      {testAvailableProviders.map(p => {
                        const providerNames: Record<string, string> = { anthropic: 'Anthropic', openai: 'OpenAI', grok: 'Grok (xAI)', gemini: 'Gemini' }
                        const checked = testSelectedProviders.includes(p.provider)
                        const providerModelDefs = testProviderModels.find(pm => pm.id === p.provider)
                        return (
                          <div key={p.provider} className={`p-3 rounded-lg border transition-all ${
                            checked ? 'bg-primary-500/10 border-primary-500/30' : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                          }`}>
                            <label className="flex items-center gap-2 cursor-pointer">
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={e => {
                                  if (e.target.checked) setTestSelectedProviders(prev => [...prev, p.provider])
                                  else setTestSelectedProviders(prev => prev.filter(id => id !== p.provider))
                                }}
                                className="accent-primary-500"
                              />
                              <span className="text-white text-sm font-medium">{providerNames[p.provider] || p.provider}</span>
                            </label>
                            {checked && providerModelDefs && providerModelDefs.models.length > 1 && (
                              <select
                                value={testModelSelections[p.provider] || ''}
                                onChange={e => setTestModelSelections(prev => ({ ...prev, [p.provider]: e.target.value }))}
                                className="mt-2 w-full bg-dark-800 border border-dark-600 text-dark-300 text-xs rounded-md px-2 py-1.5 focus:border-primary-500/50 focus:outline-none cursor-pointer"
                              >
                                {providerModelDefs.models.map(m => (
                                  <option key={m.id} value={m.id}>{m.name}</option>
                                ))}
                              </select>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>

                {/* Query editor */}
                <div className="bg-dark-800/50 border border-dark-700 rounded-xl p-4">
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-white font-semibold text-sm">Test Queries</h3>
                    <button
                      onClick={() => setTestQueries(prev => [...prev, { query: '', type: 'custom', priority: 'medium' }])}
                      className="text-xs px-2 py-1 bg-dark-700 text-dark-300 rounded hover:text-white transition-colors cursor-pointer"
                    >+ Add Query</button>
                  </div>
                  {testQueries.length === 0 ? (
                    <p className="text-dark-500 text-sm">No queries yet. Add queries or <button onClick={() => {
                      apiFetch('/api/test/generate-queries', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ domain: selectedDomain }),
                      }).then(r => r.ok ? r.json() : null).then(data => {
                        if (data?.queries) setTestQueries(data.queries)
                      }).catch(() => {})
                    }} className="text-primary-400 hover:text-primary-300 cursor-pointer underline">generate from brand profile</button>.</p>
                  ) : (
                    <div className="space-y-2">
                      {testQueries.map((q, i) => {
                        const typeColors: Record<string, string> = {
                          brand: 'bg-primary-500/20 text-primary-300',
                          category: 'bg-accent-purple/20 text-purple-300',
                          comparison: 'bg-accent-cyan/20 text-cyan-300',
                          discovery: 'bg-accent-emerald/20 text-emerald-300',
                          custom: 'bg-dark-700 text-dark-300',
                        }
                        return (
                          <div key={i} className="flex items-center gap-2">
                            <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium shrink-0 ${typeColors[q.type] || typeColors.custom}`}>{q.type}</span>
                            <input
                              type="text"
                              value={q.query}
                              onChange={e => setTestQueries(prev => prev.map((qq, j) => j === i ? { ...qq, query: e.target.value } : qq))}
                              className="flex-1 bg-dark-900/50 border border-dark-700 rounded-lg px-3 py-1.5 text-white text-sm placeholder-dark-500 focus:outline-none focus:border-primary-500/50"
                              placeholder="Enter a question..."
                            />
                            <select
                              value={q.type}
                              onChange={e => setTestQueries(prev => prev.map((qq, j) => j === i ? { ...qq, type: e.target.value } : qq))}
                              className="bg-dark-900 border border-dark-700 rounded px-2 py-1.5 text-dark-300 text-xs cursor-pointer"
                            >
                              <option value="brand">Brand</option>
                              <option value="category">Category</option>
                              <option value="comparison">Comparison</option>
                              <option value="discovery">Discovery</option>
                              <option value="custom">Custom</option>
                            </select>
                            <button
                              onClick={() => setTestQueries(prev => prev.filter((_, j) => j !== i))}
                              className="text-dark-500 hover:text-red-400 transition-colors cursor-pointer shrink-0"
                              title="Remove query"
                            >
                              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
                            </button>
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>

                {/* Run button */}
                <div className="flex justify-end">
                  <button
                    onClick={() => runLLMTest(true)}
                    disabled={testSelectedProviders.length === 0 || testQueries.length === 0 || testQueries.some(q => !q.query.trim())}
                    className={`px-6 py-2.5 rounded-xl text-sm font-semibold transition-all ${
                      testSelectedProviders.length > 0 && testQueries.length > 0 && !testQueries.some(q => !q.query.trim())
                        ? 'bg-gradient-to-r from-primary-600 to-primary-500 text-white hover:from-primary-500 hover:to-primary-400 cursor-pointer shadow-lg shadow-primary-500/20'
                        : 'bg-dark-800 text-dark-500 cursor-not-allowed'
                    }`}
                  >
                    Run Test ({testSelectedProviders.length} provider{testSelectedProviders.length !== 1 ? 's' : ''}, {testQueries.filter(q => q.query.trim()).length} queries)
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </main>

      {/* Delete test confirmation modal */}
      {confirmDeleteTest && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Test Results</h3>
            <p className="text-dark-400 text-sm mb-6">Are you sure you want to delete the LLM test results for <span className="text-white font-medium">{selectedDomain}</span>?</p>
            <div className="flex justify-end gap-3">
              <button onClick={() => setConfirmDeleteTest(false)} className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer">Cancel</button>
              <button onClick={deleteLLMTest} className="px-4 py-2 bg-red-600 text-white text-sm font-medium rounded-lg hover:bg-red-500 transition-colors cursor-pointer">Delete</button>
            </div>
          </div>
        </div>
      )}

      {/* Competitor test modal */}
      {showCompetitorModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Test Competitor</h3>
            <p className="text-dark-400 text-sm mb-4">Run the same test queries against a competitor to compare LLM visibility.</p>
            <input
              type="text"
              placeholder="competitor.com"
              value={competitorDomain}
              onChange={e => setCompetitorDomain(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && competitorDomain.trim()) runCompetitorTest(competitorDomain.trim()) }}
              className="w-full px-3 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white text-sm placeholder-dark-500 focus:outline-none focus:border-primary-500 mb-4"
              autoFocus
            />
            <div className="flex justify-end gap-3">
              <button onClick={() => { setShowCompetitorModal(false); setCompetitorDomain('') }} className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer">Cancel</button>
              <button
                onClick={() => { if (competitorDomain.trim()) runCompetitorTest(competitorDomain.trim()) }}
                disabled={!competitorDomain.trim()}
                className="px-4 py-2 bg-amber-600 text-white text-sm font-medium rounded-lg hover:bg-amber-500 transition-colors cursor-pointer disabled:opacity-50"
              >
                Test
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete search analysis confirmation modal */}
      {confirmDeleteSearchAnalysis && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Search Analysis</h3>
            <p className="text-dark-400 text-sm mb-6">Permanently delete the Search Visibility analysis for <span className="text-white font-medium">{selectedDomain}</span>? This cannot be undone.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteSearchAnalysis(false)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={deleteSearchAnalysis}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete reddit analysis confirmation modal */}
      {confirmDeleteRedditAnalysis && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Reddit Analysis</h3>
            <p className="text-dark-400 text-sm mb-6">Permanently delete the Reddit analysis for <span className="text-white font-medium">{redditDomain}</span>? This cannot be undone.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteRedditAnalysis(false)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={deleteRedditAnalysis}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete video analysis confirmation modal */}
      {confirmDeleteVideoAnalysis && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Video Analysis</h3>
            <p className="text-dark-400 text-sm mb-6">Permanently delete the video analysis for <span className="text-white font-medium">{videoDomain}</span>? This cannot be undone.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteVideoAnalysis(false)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={deleteVideoAnalysis}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete brand confirmation modal */}
      {confirmDeleteBrand && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Brand Profile</h3>
            <p className="text-dark-400 text-sm mb-6">Permanently delete the brand profile for <span className="text-white font-medium">{brandDomain}</span>? This cannot be undone.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteBrand(false)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  setConfirmDeleteBrand(false)
                  if (!brandDomain) return
                  try {
                    await apiFetch(`/api/brands/${encodeURIComponent(brandDomain)}`, { method: 'DELETE' })
                    setBrandEditing(false)
                    setBrandProfile(null)
                    fetchBrandList()
                  } catch { /* ignore */ }
                }}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete analysis confirmation modal */}
      {confirmDeleteAnalysis && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Analysis</h3>
            <p className="text-dark-400 text-sm mb-6">This will permanently delete this analysis along with all associated optimization reports and to-do items.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteAnalysis(null)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => deleteAnalysis(confirmDeleteAnalysis)}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete optimization confirmation modal */}
      {confirmDeleteOptimization && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Delete Report</h3>
            <p className="text-dark-400 text-sm mb-6">This will permanently delete this optimization report and all associated to-do items.</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDeleteOptimization(null)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => deleteOptimization(confirmDeleteOptimization)}
                className="flex-1 px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-500 transition-colors cursor-pointer font-medium"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Unsaved brand changes modal */}
      {pendingNavAction && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-sm w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-2">Unsaved Changes</h3>
            <p className="text-dark-400 text-sm mb-6">You have unsaved changes to this brand profile. What would you like to do?</p>
            <div className="flex gap-3">
              <button
                onClick={() => {
                  brandDirtyRef.current = false
                  const action = pendingNavAction
                  setPendingNavAction(null)
                  action()
                }}
                className="flex-1 px-4 py-2 text-sm text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/10 transition-colors cursor-pointer"
              >
                Discard
              </button>
              <button
                onClick={() => setPendingNavAction(null)}
                className="flex-1 px-4 py-2 text-sm text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  const action = pendingNavAction
                  setPendingNavAction(null)
                  await saveBrandProfile()
                  action()
                }}
                className="flex-1 px-4 py-2 text-sm text-white bg-primary-600 rounded-lg hover:bg-primary-500 transition-colors cursor-pointer font-medium"
              >
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Share Domain Modal */}
      {shareModalDomain && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-white font-semibold text-lg">Share Domain</h3>
              <button
                onClick={() => setShareModalDomain(null)}
                className="text-dark-400 hover:text-white transition-colors cursor-pointer"
              >
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>
            <p className="text-dark-400 text-sm mb-4">
              Control who can view reports for <span className="text-white font-medium">{shareModalDomain}</span>.
              All reports (analysis, optimization, video, todos) are shared collectively.
            </p>

            {shareLoading && !domainShareState ? (
              <div className="text-center py-6 text-dark-400">Loading...</div>
            ) : (
              <div className="space-y-3">
                {/* Private option */}
                <button
                  onClick={() => setDomainVisibility(shareModalDomain, 'private')}
                  disabled={shareLoading}
                  className={`w-full text-left p-3 rounded-xl border transition-all cursor-pointer ${
                    domainShareState?.visibility === 'private'
                      ? 'bg-dark-800 border-primary-500/50'
                      : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                      domainShareState?.visibility === 'private' ? 'border-primary-500' : 'border-dark-600'
                    }`}>
                      {domainShareState?.visibility === 'private' && <div className="w-2 h-2 rounded-full bg-primary-500" />}
                    </div>
                    <div>
                      <p className="text-white text-sm font-medium">Private</p>
                      <p className="text-dark-500 text-xs">Only team members can view these reports</p>
                    </div>
                  </div>
                </button>

                {/* Public option */}
                <button
                  onClick={() => setDomainVisibility(shareModalDomain, 'public')}
                  disabled={shareLoading}
                  className={`w-full text-left p-3 rounded-xl border transition-all cursor-pointer ${
                    domainShareState?.visibility === 'public'
                      ? 'bg-dark-800 border-primary-500/50'
                      : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                      domainShareState?.visibility === 'public' ? 'border-primary-500' : 'border-dark-600'
                    }`}>
                      {domainShareState?.visibility === 'public' && <div className="w-2 h-2 rounded-full bg-primary-500" />}
                    </div>
                    <div>
                      <p className="text-white text-sm font-medium">Public</p>
                      <p className="text-dark-500 text-xs">Anyone with the link can view these reports</p>
                    </div>
                  </div>
                </button>

                {/* Popular option (root tenant admin/owner only) */}
                {user?.isRootTenant && (user.role === 'owner' || user.role === 'admin') && (
                  <button
                    onClick={() => setDomainVisibility(shareModalDomain, 'popular')}
                    disabled={shareLoading}
                    className={`w-full text-left p-3 rounded-xl border transition-all cursor-pointer ${
                      domainShareState?.visibility === 'popular'
                        ? 'bg-dark-800 border-accent-purple/50'
                        : 'bg-dark-900/50 border-dark-700 hover:border-dark-600'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <div className={`w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                        domainShareState?.visibility === 'popular' ? 'border-accent-purple' : 'border-dark-600'
                      }`}>
                        {domainShareState?.visibility === 'popular' && <div className="w-2 h-2 rounded-full bg-accent-purple" />}
                      </div>
                      <div>
                        <p className="text-white text-sm font-medium">Popular</p>
                        <p className="text-dark-500 text-xs">Featured on the Analyze page + publicly accessible</p>
                      </div>
                    </div>
                  </button>
                )}

                {/* Share URL display */}
                {domainShareState?.share_url && (domainShareState.visibility === 'public' || domainShareState.visibility === 'popular') && (() => {
                  const shareFullUrl = `${window.location.origin}${domainShareState.share_url}?tab=${activeTab}`
                  const shareText = `Check out this LLM visibility report for ${shareModalDomain}`
                  return (
                  <div className="mt-4 p-3 bg-dark-800 rounded-xl border border-dark-700">
                    <label className="text-dark-500 text-xs font-medium block mb-2">Share Link</label>
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        readOnly
                        value={shareFullUrl}
                        className="flex-1 bg-dark-900 border border-dark-700 rounded-lg px-3 py-2 text-sm text-dark-300 font-mono"
                      />
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(shareFullUrl)
                          trackEvent('custom.share.link_copied', { domain: shareModalDomain })
                          setShareCopied(true)
                          setTimeout(() => setShareCopied(false), 2000)
                        }}
                        className="px-3 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-500 transition-colors cursor-pointer font-medium shrink-0"
                      >
                        {shareCopied ? 'Copied!' : 'Copy'}
                      </button>
                    </div>
                    {/* Social share buttons */}
                    <div className="flex items-center gap-2 mt-3">
                      <span className="text-dark-500 text-xs">Share on:</span>
                      <a
                        href={`https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(shareFullUrl)}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="px-2.5 py-1 text-xs bg-[#0A66C2]/20 text-[#0A66C2] rounded-lg hover:bg-[#0A66C2]/30 transition-colors font-medium"
                      >
                        LinkedIn
                      </a>
                      <a
                        href={`https://twitter.com/intent/tweet?url=${encodeURIComponent(shareFullUrl)}&text=${encodeURIComponent(shareText)}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="px-2.5 py-1 text-xs bg-dark-700 text-dark-300 rounded-lg hover:bg-dark-600 transition-colors font-medium"
                      >
                        X / Twitter
                      </a>
                    </div>
                    {/* View count */}
                    {(domainShareState.view_count ?? 0) > 0 && (
                      <div className="mt-3 text-dark-500 text-xs flex items-center gap-1.5">
                        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                          <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                        </svg>
                        Viewed {domainShareState.view_count?.toLocaleString()} {domainShareState.view_count === 1 ? 'time' : 'times'}
                      </div>
                    )}
                  </div>
                  )
                })()}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Optimize Confirmation Modal — shown when clicking a question with no existing report */}
      {optimizeConfirmQ !== null && result && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Generate Optimization Report</h3>
            <p className="text-dark-400 text-sm mb-6">
              Do you want to generate an Optimize report to help surface this question to LLMs at <span className="text-white font-medium">{brandList.find(b => domainKey(b.domain) === domainKey(selectedDomain))?.brand_name || selectedDomain}</span> better?
            </p>
            <p className="text-dark-500 text-xs mb-6 italic">
              &ldquo;{result.questions[optimizeConfirmQ]?.question}&rdquo;
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setOptimizeConfirmQ(null)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => { const qi = optimizeConfirmQ; setOptimizeConfirmQ(null); optimizeQuestion(qi) }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                Yes, Generate
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ReadOnly "Not Ready" Modal — shown when clicking question with no optimize report in shared view */}
      {readOnlyOptModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Report Not Available</h3>
            <p className="text-dark-400 text-sm mb-6">
              An Optimize report for this question hasn't been generated yet. Would you like to see other optimization reports for <span className="text-white font-medium">{readOnlyOptModal}</span>?
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setReadOnlyOptModal(null)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                No, Thanks
              </button>
              <button
                onClick={() => { setReadOnlyOptModal(null); setActiveTab('optimize') }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                View Reports
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Login Required Modal */}
      {loginModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Sign In Required</h3>
            <p className="text-dark-400 text-sm mb-6">
              You need to be signed in to generate new analyses. You can browse the popular domains below for free without an account.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setLoginModal(false)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => { setLoginModal(false); window.location.href = '/login' }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                Sign In / Sign Up
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Subscription Required Modal */}
      {subscriptionModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-white font-semibold text-lg mb-3">Subscription Required</h3>
            <p className="text-dark-400 text-sm mb-4">
              You need an active subscription to run reports. You can still view popular public reports shared by other users.
            </p>
            <p className="text-dark-400 text-sm mb-6">
              Would you like to choose a plan?
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setSubscriptionModal(false)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Not Now
              </button>
              <button
                onClick={() => { setSubscriptionModal(false); window.location.href = '/last/plan' }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                View Plans
              </button>
            </div>
          </div>
        </div>
      )}

      {apiKeyModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <div className="flex items-center gap-3 mb-3">
              <div className="w-10 h-10 rounded-full bg-amber-500/10 flex items-center justify-center">
                <svg className="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" /></svg>
              </div>
              <h3 className="text-white font-semibold text-lg">API Key Required</h3>
            </div>
            <p className="text-dark-400 text-sm mb-6">
              {userTenantRole === 'owner'
                ? 'You need to configure an API key before generating reports. Set up your AI provider API key in Settings to get started.'
                : 'Your team needs an API key configured before you can generate reports. Ask your team owner to set up an API key in Settings.'}
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setApiKeyModal(false)}
                className="px-4 py-2 text-dark-400 hover:text-white text-sm transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => { setApiKeyModal(false); window.location.href = '/last/settings' }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                {userTenantRole === 'owner' ? 'Set Keys' : 'View Settings'}
              </button>
            </div>
          </div>
        </div>
      )}

      {welcomeKeyModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md w-full mx-4 shadow-2xl">
            <div className="flex items-center gap-3 mb-3">
              <div className="w-10 h-10 rounded-full bg-primary-500/10 flex items-center justify-center">
                <svg className="w-5 h-5 text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}><path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z" /></svg>
              </div>
              <h3 className="text-white font-semibold text-lg">Welcome! One More Step</h3>
            </div>
            <p className="text-dark-400 text-sm mb-2">
              To start generating AI-powered reports, you'll need to connect an API key from an AI provider (Anthropic, OpenAI, Grok, or Gemini).
            </p>
            <p className="text-dark-500 text-xs mb-6">
              You can configure your API keys in Settings. Most providers offer free tiers to get started.
            </p>
            <div className="flex justify-end">
              <button
                onClick={() => { setWelcomeKeyModal(false); window.location.href = '/last/settings' }}
                className="px-4 py-2 bg-gradient-to-r from-primary-600 to-primary-500 text-white text-sm font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all cursor-pointer"
              >
                Set Up API Key
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Powered by banner — shared/read-only views */}
      {readOnly && (
        <div className="sticky bottom-0 z-40 bg-gradient-to-r from-primary-600/95 to-accent-purple/95 backdrop-blur-sm border-t border-primary-500/30">
          <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-3 flex items-center justify-between">
            <div className="flex items-center gap-2 text-white/90 text-sm">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
              </svg>
              <span>Powered by <strong>LLM Optimizer</strong> &middot; <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer" className="underline underline-offset-2 hover:text-white">Open Source</a></span>
            </div>
            <a href="/signup" className="px-4 py-1.5 bg-white text-primary-700 text-sm font-semibold rounded-lg hover:bg-white/90 transition-colors">
              Analyze Your Brand
            </a>
          </div>
        </div>
      )}

      {/* Site Footer */}
      <footer className="border-t border-dark-700 bg-dark-900/80 py-8 mt-12">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col items-center gap-4 text-xs">
            <div className="flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-dark-400">
              <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">GitHub (MIT License)</a>
              <span className="text-dark-700">·</span>
              <a href="https://www.metavert.io/privacy-policy" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Privacy Policy</a>
              <span className="text-dark-700">·</span>
              <a href="https://www.metavert.io/terms-of-service" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">Terms of Service</a>
              <span className="text-dark-700">·</span>
              <a href="/research" className="hover:text-white transition-colors">Research Citations</a>
              <span className="text-dark-700">·</span>
              <a href="/docs" className="hover:text-white transition-colors">API Docs</a>
              <span className="text-dark-700">·</span>
              <a href="/api/v1/docs" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">API Reference (MD)</a>
            </div>
            <p className="text-dark-500">&copy; 2026 Metavert LLC. All content licensed under <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer" className="text-dark-400 hover:text-white transition-colors">Creative Commons Attribution 4.0</a>.</p>
          </div>
        </div>
      </footer>
    </div>
  )
}
