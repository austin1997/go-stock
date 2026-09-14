// Generated from frontend/wailsjs/go/main/App.js — do not edit.
import { rpc } from './rpc.js'

export function AbortChatWithAgent() {
  return rpc('AbortChatWithAgent');
}

export function AbortSummaryStockNews() {
  return rpc('AbortSummaryStockNews');
}

export function AddAllStockInfo(arg1) {
  return rpc('AddAllStockInfo', arg1);
}

export function AddConcept(arg1) {
  return rpc('AddConcept', arg1);
}

export function AddCronTask(arg1) {
  return rpc('AddCronTask', arg1);
}

export function AddGroup(arg1) {
  return rpc('AddGroup', arg1);
}

export function AddKBDocument(arg1, arg2, arg3) {
  return rpc('AddKBDocument', arg1, arg2, arg3);
}

export function AddPrompt(arg1) {
  return rpc('AddPrompt', arg1);
}

export function AddPromptTemplate(arg1) {
  return rpc('AddPromptTemplate', arg1);
}

export function AddStockConcept(arg1, arg2) {
  return rpc('AddStockConcept', arg1, arg2);
}

export function AddStockGroup(arg1, arg2) {
  return rpc('AddStockGroup', arg1, arg2);
}

export function AddTradingRecord(arg1) {
  return rpc('AddTradingRecord', arg1);
}

export function AnalyzeSentiment(arg1) {
  return rpc('AnalyzeSentiment', arg1);
}

export function AnalyzeSentimentWithFreqWeight(arg1) {
  return rpc('AnalyzeSentimentWithFreqWeight', arg1);
}

export function BatchDeleteAIResponseResult(arg1) {
  return rpc('BatchDeleteAIResponseResult', arg1);
}

export function BatchDeleteAllStockInfo(arg1) {
  return rpc('BatchDeleteAllStockInfo', arg1);
}

export function BuildKBGraph(arg1, arg2) {
  return rpc('BuildKBGraph', arg1, arg2);
}

export function CalculateNextRunTime(arg1) {
  return rpc('CalculateNextRunTime', arg1);
}

export function CalculateNextRunTimes(arg1, arg2) {
  return rpc('CalculateNextRunTimes', arg1, arg2);
}

export function ChatWithAgent(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10) {
  return rpc('ChatWithAgent', arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10);
}

export function ChatWithAgentKBQA(arg1, arg2, arg3, arg4) {
  return rpc('ChatWithAgentKBQA', arg1, arg2, arg3, arg4);
}

export function CheckDeviceBinding(arg1, arg2) {
  return rpc('CheckDeviceBinding', arg1, arg2);
}

export function CheckFrequentTrading(arg1) {
  return rpc('CheckFrequentTrading', arg1);
}

export function CheckSponsorCode(arg1) {
  return rpc('CheckSponsorCode', arg1);
}

export function CheckStockBaseInfo(arg1) {
  return rpc('CheckStockBaseInfo', arg1);
}

export function CheckUpdate(arg1) {
  return rpc('CheckUpdate', arg1);
}

export function ClearAgentFeedback() {
  return rpc('ClearAgentFeedback');
}

export function ClearUserProfile() {
  return rpc('ClearUserProfile');
}

export function ClsCalendar() {
  return rpc('ClsCalendar');
}

export function ConceptDetail(arg1) {
  return rpc('ConceptDetail', arg1);
}

export function ConceptEventList(arg1) {
  return rpc('ConceptEventList', arg1);
}

export function ConceptKLine(arg1) {
  return rpc('ConceptKLine', arg1);
}

export function ConceptRealHead(arg1) {
  return rpc('ConceptRealHead', arg1);
}

export function ConceptStocks(arg1, arg2) {
  return rpc('ConceptStocks', arg1, arg2);
}

export function CreateCronTask(arg1) {
  return rpc('CreateCronTask', arg1);
}

export function CreateKnowledgeBase(arg1, arg2, arg3, arg4) {
  return rpc('CreateKnowledgeBase', arg1, arg2, arg3, arg4);
}

export function CreateMCPServer(arg1) {
  return rpc('CreateMCPServer', arg1);
}

export function CreateSkill(arg1) {
  return rpc('CreateSkill', arg1);
}

export function DelPrompt(arg1) {
  return rpc('DelPrompt', arg1);
}

export function DeleteAIResponseResult(arg1) {
  return rpc('DeleteAIResponseResult', arg1);
}

export function DeleteAgentFeedback(arg1) {
  return rpc('DeleteAgentFeedback', arg1);
}

export function DeleteAiRecommendStocks(arg1) {
  return rpc('DeleteAiRecommendStocks', arg1);
}

export function DeleteAllStockInfo(arg1) {
  return rpc('DeleteAllStockInfo', arg1);
}

export function DeleteCronTask(arg1) {
  return rpc('DeleteCronTask', arg1);
}

export function DeleteCustomStrategy(arg1) {
  return rpc('DeleteCustomStrategy', arg1);
}

export function DeleteDailyOperationPlan(arg1) {
  return rpc('DeleteDailyOperationPlan', arg1);
}

export function DeleteDailyReview(arg1) {
  return rpc('DeleteDailyReview', arg1);
}

export function DeleteFilesystemSkill(arg1) {
  return rpc('DeleteFilesystemSkill', arg1);
}

export function DeleteKBDocument(arg1, arg2) {
  return rpc('DeleteKBDocument', arg1, arg2);
}

export function DeleteKBGraph(arg1) {
  return rpc('DeleteKBGraph', arg1);
}

export function DeleteKnowledgeBase(arg1) {
  return rpc('DeleteKnowledgeBase', arg1);
}

export function DeleteMCPServer(arg1) {
  return rpc('DeleteMCPServer', arg1);
}

export function DeleteMorningStrategy(arg1) {
  return rpc('DeleteMorningStrategy', arg1);
}

export function DeletePromptTemplate(arg1) {
  return rpc('DeletePromptTemplate', arg1);
}

export function DeleteSkill(arg1) {
  return rpc('DeleteSkill', arg1);
}

export function DeleteSkillFile(arg1, arg2) {
  return rpc('DeleteSkillFile', arg1, arg2);
}

export function DeleteStockChangeHistory(arg1) {
  return rpc('DeleteStockChangeHistory', arg1);
}

export function DeleteTradingRecord(arg1) {
  return rpc('DeleteTradingRecord', arg1);
}

export function DisableFilesystemSkill(arg1) {
  return rpc('DisableFilesystemSkill', arg1);
}

export function EMDictCode(arg1) {
  return rpc('EMDictCode', arg1);
}

export function EnableCronTask(arg1, arg2) {
  return rpc('EnableCronTask', arg1, arg2);
}

export function EnableFilesystemSkill(arg1) {
  return rpc('EnableFilesystemSkill', arg1);
}

export function EnableMCPServer(arg1, arg2) {
  return rpc('EnableMCPServer', arg1, arg2);
}

export function EnableSkill(arg1, arg2) {
  return rpc('EnableSkill', arg1, arg2);
}

export function ExecuteCronTaskNow(arg1) {
  return rpc('ExecuteCronTaskNow', arg1);
}

export function ExportConfig() {
  return rpc('ExportConfig');
}

export function ExportTableToXLSX(arg1, arg2) {
  return rpc('ExportTableToXLSX', arg1, arg2);
}

export function ExportTradingRecordTemplate() {
  return rpc('ExportTradingRecordTemplate');
}

export function FetchAiModelInfo(arg1, arg2, arg3, arg4) {
  return rpc('FetchAiModelInfo', arg1, arg2, arg3, arg4);
}

export function FetchAiModels(arg1, arg2, arg3) {
  return rpc('FetchAiModels', arg1, arg2, arg3);
}

export function FetchAndSaveMarketStatistic() {
  return rpc('FetchAndSaveMarketStatistic');
}

export function FindConceptCodeByName(arg1) {
  return rpc('FindConceptCodeByName', arg1);
}

export function Follow(arg1) {
  return rpc('Follow', arg1);
}

export function FollowFund(arg1) {
  return rpc('FollowFund', arg1);
}

export function GenerateDailyReviewNow(arg1, arg2, arg3, arg4) {
  return rpc('GenerateDailyReviewNow', arg1, arg2, arg3, arg4);
}

export function GenerateMorningStrategyNow(arg1, arg2, arg3, arg4) {
  return rpc('GenerateMorningStrategyNow', arg1, arg2, arg3, arg4);
}

export function GetAIResponseResult(arg1) {
  return rpc('GetAIResponseResult', arg1);
}

export function GetAIResponseResultList(arg1) {
  return rpc('GetAIResponseResultList', arg1);
}

export function GetAgentFeedbackStats() {
  return rpc('GetAgentFeedbackStats');
}

export function GetAiAssistantSession(arg1) {
  return rpc('GetAiAssistantSession', arg1);
}

export function GetAiConfigs() {
  return rpc('GetAiConfigs');
}

export function GetAiRecommendStocksList(arg1) {
  return rpc('GetAiRecommendStocksList', arg1);
}

export function GetAllBKCodes() {
  return rpc('GetAllBKCodes');
}

export function GetAllConceptCodes() {
  return rpc('GetAllConceptCodes');
}

export function GetAllConceptPlates() {
  return rpc('GetAllConceptPlates');
}

export function GetAllConcepts() {
  return rpc('GetAllConcepts');
}

export function GetAllCustomStrategies() {
  return rpc('GetAllCustomStrategies');
}

export function GetAllDeptPolicyNews(arg1) {
  return rpc('GetAllDeptPolicyNews', arg1);
}

export function GetAllGroupStocks() {
  return rpc('GetAllGroupStocks');
}

export function GetAllIndustries() {
  return rpc('GetAllIndustries');
}

export function GetAllIndustryPlates() {
  return rpc('GetAllIndustryPlates');
}

export function GetAllKBVectorizingStatuses() {
  return rpc('GetAllKBVectorizingStatuses');
}

export function GetAllMCPTools() {
  return rpc('GetAllMCPTools');
}

export function GetAllMarkets() {
  return rpc('GetAllMarkets');
}

export function GetAllSkills() {
  return rpc('GetAllSkills');
}

export function GetAllStockChangesWithPaging(arg1) {
  return rpc('GetAllStockChangesWithPaging', arg1);
}

export function GetAllStockConcepts() {
  return rpc('GetAllStockConcepts');
}

export function GetAllStockInfoById(arg1) {
  return rpc('GetAllStockInfoById', arg1);
}

export function GetAllStockInfoList(arg1) {
  return rpc('GetAllStockInfoList', arg1);
}

export function GetAllStocks(arg1, arg2, arg3, arg4) {
  return rpc('GetAllStocks', arg1, arg2, arg3, arg4);
}

export function GetAllTdxTransactionData(arg1) {
  return rpc('GetAllTdxTransactionData', arg1);
}

export function GetBKConstituentStocks(arg1) {
  return rpc('GetBKConstituentStocks', arg1);
}

export function GetBKFundFlowList(arg1, arg2) {
  return rpc('GetBKFundFlowList', arg1, arg2);
}

export function GetBKFundFlowListByDate(arg1, arg2) {
  return rpc('GetBKFundFlowListByDate', arg1, arg2);
}

export function GetBKFundFlowTopList(arg1) {
  return rpc('GetBKFundFlowTopList', arg1);
}

export function GetBKFundFlowTopListByDate(arg1, arg2) {
  return rpc('GetBKFundFlowTopListByDate', arg1, arg2);
}

export function GetChangeRank(arg1, arg2) {
  return rpc('GetChangeRank', arg1, arg2);
}

export function GetChangeTypeDailyStats(arg1) {
  return rpc('GetChangeTypeDailyStats', arg1);
}

export function GetChipDistribution(arg1, arg2, arg3, arg4) {
  return rpc('GetChipDistribution', arg1, arg2, arg3, arg4);
}

export function GetConceptFundFlowList(arg1, arg2) {
  return rpc('GetConceptFundFlowList', arg1, arg2);
}

export function GetConceptFundFlowListByDate(arg1, arg2) {
  return rpc('GetConceptFundFlowListByDate', arg1, arg2);
}

export function GetConceptFundFlowTopList(arg1) {
  return rpc('GetConceptFundFlowTopList', arg1);
}

export function GetConceptFundFlowTopListByDate(arg1, arg2) {
  return rpc('GetConceptFundFlowTopListByDate', arg1, arg2);
}

export function GetConceptList() {
  return rpc('GetConceptList');
}

export function GetConfig() {
  return rpc('GetConfig');
}

export function GetCronTaskByID(arg1) {
  return rpc('GetCronTaskByID', arg1);
}

export function GetCronTaskList(arg1) {
  return rpc('GetCronTaskList', arg1);
}

export function GetCronTaskTypes() {
  return rpc('GetCronTaskTypes');
}

export function GetCustomStrategyList(arg1) {
  return rpc('GetCustomStrategyList', arg1);
}

export function GetDailyChangeStats(arg1) {
  return rpc('GetDailyChangeStats', arg1);
}

export function GetDailyDimensionStats(arg1, arg2, arg3) {
  return rpc('GetDailyDimensionStats', arg1, arg2, arg3);
}

export function GetDailyOperationPlanByID(arg1) {
  return rpc('GetDailyOperationPlanByID', arg1);
}

export function GetDailyOperationPlanList(arg1) {
  return rpc('GetDailyOperationPlanList', arg1);
}

export function GetDailyReviewByDate(arg1) {
  return rpc('GetDailyReviewByDate', arg1);
}

export function GetDailyReviewList(arg1, arg2) {
  return rpc('GetDailyReviewList', arg1, arg2);
}

export function GetEffectiveSponsorVip() {
  return rpc('GetEffectiveSponsorVip');
}

export function GetFeishuBotStatus() {
  return rpc('GetFeishuBotStatus');
}

export function GetFollowList(arg1) {
  return rpc('GetFollowList', arg1);
}

export function GetFollowedFund() {
  return rpc('GetFollowedFund');
}

export function GetFollowedFundPaged(arg1, arg2, arg3) {
  return rpc('GetFollowedFundPaged', arg1, arg2, arg3);
}

export function GetFundHistoryNetValue(arg1, arg2, arg3, arg4) {
  return rpc('GetFundHistoryNetValue', arg1, arg2, arg3, arg4);
}

export function GetFundKLine(arg1, arg2, arg3) {
  return rpc('GetFundKLine', arg1, arg2, arg3);
}

export function GetFundRanking(arg1, arg2, arg3, arg4, arg5, arg6) {
  return rpc('GetFundRanking', arg1, arg2, arg3, arg4, arg5, arg6);
}

export function GetFundTop10Holdings(arg1) {
  return rpc('GetFundTop10Holdings', arg1);
}

export function GetFuturesMemberRank(arg1, arg2) {
  return rpc('GetFuturesMemberRank', arg1, arg2);
}

export function GetFuturesPositionTrend(arg1, arg2, arg3) {
  return rpc('GetFuturesPositionTrend', arg1, arg2, arg3);
}

export function GetGlobalIndexTrend(arg1) {
  return rpc('GetGlobalIndexTrend', arg1);
}

export function GetGovDepartments() {
  return rpc('GetGovDepartments');
}

export function GetGroupList() {
  return rpc('GetGroupList');
}

export function GetGroupStockList(arg1) {
  return rpc('GetGroupStockList', arg1);
}

export function GetHistoryTdxMinuteTimeData(arg1, arg2) {
  return rpc('GetHistoryTdxMinuteTimeData', arg1, arg2);
}

export function GetHistoryTdxTransactionData(arg1, arg2) {
  return rpc('GetHistoryTdxTransactionData', arg1, arg2);
}

export function GetHotMoneySeats() {
  return rpc('GetHotMoneySeats');
}

export function GetHotStrategy() {
  return rpc('GetHotStrategy');
}

export function GetIndexQuotes() {
  return rpc('GetIndexQuotes');
}

export function GetIndexTline(arg1) {
  return rpc('GetIndexTline', arg1);
}

export function GetIndustryMoneyRankSina(arg1, arg2) {
  return rpc('GetIndustryMoneyRankSina', arg1, arg2);
}

export function GetIndustryRank(arg1, arg2) {
  return rpc('GetIndustryRank', arg1, arg2);
}

export function GetKBGraph(arg1) {
  return rpc('GetKBGraph', arg1);
}

export function GetKBGraphBuildStatus(arg1) {
  return rpc('GetKBGraphBuildStatus', arg1);
}

export function GetKBVectorizingStatus(arg1) {
  return rpc('GetKBVectorizingStatus', arg1);
}

export function GetKeyDepartments() {
  return rpc('GetKeyDepartments');
}

export function GetKeyDeptPolicyNews(arg1) {
  return rpc('GetKeyDeptPolicyNews', arg1);
}

export function GetKnowledgeBase(arg1) {
  return rpc('GetKnowledgeBase', arg1);
}

export function GetKoreaDayKLine(arg1, arg2) {
  return rpc('GetKoreaDayKLine', arg1, arg2);
}

export function GetLatestDailyReview() {
  return rpc('GetLatestDailyReview');
}

export function GetLatestMorningStrategy() {
  return rpc('GetLatestMorningStrategy');
}

export function GetLatestTradingDay() {
  return rpc('GetLatestTradingDay');
}

export function GetLhbDailySummary(arg1) {
  return rpc('GetLhbDailySummary', arg1);
}

export function GetLhbSeatDetail(arg1, arg2) {
  return rpc('GetLhbSeatDetail', arg1, arg2);
}

export function GetLongTermMemoryAiConfigId() {
  return rpc('GetLongTermMemoryAiConfigId');
}

export function GetLongTermMemoryInfo() {
  return rpc('GetLongTermMemoryInfo');
}

export function GetMCPServerByID(arg1) {
  return rpc('GetMCPServerByID', arg1);
}

export function GetMCPServerList(arg1) {
  return rpc('GetMCPServerList', arg1);
}

export function GetMCPToolsByServerID(arg1) {
  return rpc('GetMCPToolsByServerID', arg1);
}

export function GetMachineId() {
  return rpc('GetMachineId');
}

export function GetMarketEmotion() {
  return rpc('GetMarketEmotion');
}

export function GetMarketStatisticByDate(arg1) {
  return rpc('GetMarketStatisticByDate', arg1);
}

export function GetMoneyRankSina(arg1) {
  return rpc('GetMoneyRankSina', arg1);
}

export function GetMorningStrategyByDate(arg1) {
  return rpc('GetMorningStrategyByDate', arg1);
}

export function GetMorningStrategyList(arg1, arg2) {
  return rpc('GetMorningStrategyList', arg1, arg2);
}

export function GetPolicyNews(arg1, arg2) {
  return rpc('GetPolicyNews', arg1, arg2);
}

export function GetProfileLearnAiConfigId() {
  return rpc('GetProfileLearnAiConfigId');
}

export function GetPromptTemplateList(arg1) {
  return rpc('GetPromptTemplateList', arg1);
}

export function GetPromptTemplates(arg1, arg2) {
  return rpc('GetPromptTemplates', arg1, arg2);
}

export function GetRecentDaysMarketStatistic(arg1) {
  return rpc('GetRecentDaysMarketStatistic', arg1);
}

export function GetRecommendBacktestStats() {
  return rpc('GetRecommendBacktestStats');
}

export function GetSectorAnchors(arg1) {
  return rpc('GetSectorAnchors', arg1);
}

export function GetSkillByID(arg1) {
  return rpc('GetSkillByID', arg1);
}

export function GetSkillList(arg1) {
  return rpc('GetSkillList', arg1);
}

export function GetSponsorInfo() {
  return rpc('GetSponsorInfo');
}

export function GetStockChangeHistory(arg1) {
  return rpc('GetStockChangeHistory', arg1);
}

export function GetStockChanges(arg1, arg2, arg3) {
  return rpc('GetStockChanges', arg1, arg2, arg3);
}

export function GetStockCommonKLine(arg1, arg2, arg3) {
  return rpc('GetStockCommonKLine', arg1, arg2, arg3);
}

export function GetStockConceptsByStockCode(arg1) {
  return rpc('GetStockConceptsByStockCode', arg1);
}

export function GetStockEastMoneyKLine(arg1, arg2, arg3, arg4) {
  return rpc('GetStockEastMoneyKLine', arg1, arg2, arg3, arg4);
}

export function GetStockEastMoneyKLinePage(arg1, arg2, arg3, arg4, arg5) {
  return rpc('GetStockEastMoneyKLinePage', arg1, arg2, arg3, arg4, arg5);
}

export function GetStockKLine(arg1, arg2, arg3) {
  return rpc('GetStockKLine', arg1, arg2, arg3);
}

export function GetStockKLinePageWithFallback(arg1, arg2, arg3, arg4, arg5, arg6) {
  return rpc('GetStockKLinePageWithFallback', arg1, arg2, arg3, arg4, arg5, arg6);
}

export function GetStockKLineWithFallback(arg1, arg2, arg3, arg4, arg5) {
  return rpc('GetStockKLineWithFallback', arg1, arg2, arg3, arg4, arg5);
}

export function GetStockList(arg1) {
  return rpc('GetStockList', arg1);
}

export function GetStockMinutePriceLineData(arg1, arg2) {
  return rpc('GetStockMinutePriceLineData', arg1, arg2);
}

export function GetStockMoneyTrendByDay(arg1, arg2) {
  return rpc('GetStockMoneyTrendByDay', arg1, arg2);
}

export function GetStockRealTimePrice(arg1) {
  return rpc('GetStockRealTimePrice', arg1);
}

export function GetStoredPolicyNews(arg1, arg2, arg3, arg4) {
  return rpc('GetStoredPolicyNews', arg1, arg2, arg3, arg4);
}

export function GetTdxCallAuction(arg1, arg2, arg3) {
  return rpc('GetTdxCallAuction', arg1, arg2, arg3);
}

export function GetTdxCompanyCategoryContent(arg1, arg2) {
  return rpc('GetTdxCompanyCategoryContent', arg1, arg2);
}

export function GetTdxCompanyCategoryList(arg1) {
  return rpc('GetTdxCompanyCategoryList', arg1);
}

export function GetTdxCompanyInfo(arg1) {
  return rpc('GetTdxCompanyInfo', arg1);
}

export function GetTdxFinanceInfo(arg1) {
  return rpc('GetTdxFinanceInfo', arg1);
}

export function GetTdxMinuteTimeData(arg1) {
  return rpc('GetTdxMinuteTimeData', arg1);
}

export function GetTdxSymbolBelongBoard(arg1) {
  return rpc('GetTdxSymbolBelongBoard', arg1);
}

export function GetTdxTransactionData(arg1, arg2, arg3) {
  return rpc('GetTdxTransactionData', arg1, arg2, arg3);
}

export function GetTdxXDXRInfo(arg1) {
  return rpc('GetTdxXDXRInfo', arg1);
}

export function GetTelegraphList(arg1) {
  return rpc('GetTelegraphList', arg1);
}

export function GetTimezone() {
  return rpc('GetTimezone');
}

export function GetTodayMarketStatistic() {
  return rpc('GetTodayMarketStatistic');
}

export function GetTradingRecordById(arg1) {
  return rpc('GetTradingRecordById', arg1);
}

export function GetTradingRecordList(arg1) {
  return rpc('GetTradingRecordList', arg1);
}

export function GetTradingRecordStatistics() {
  return rpc('GetTradingRecordStatistics');
}

export function GetTypeStatsByDate(arg1) {
  return rpc('GetTypeStatsByDate', arg1);
}

export function GetUplimitHot(arg1, arg2) {
  return rpc('GetUplimitHot', arg1, arg2);
}

export function GetUserManual() {
  return rpc('GetUserManual');
}

export function GetUserProfile() {
  return rpc('GetUserProfile');
}

export function GetUserProfileEnabled() {
  return rpc('GetUserProfileEnabled');
}

export function GetUserProfileSnapshot() {
  return rpc('GetUserProfileSnapshot');
}

export function GetUserProfileUpdatedAt() {
  return rpc('GetUserProfileUpdatedAt');
}

export function GetVersionInfo() {
  return rpc('GetVersionInfo');
}

export function GetfundList(arg1) {
  return rpc('GetfundList', arg1);
}

export function GlobalStockIndexes() {
  return rpc('GlobalStockIndexes');
}

export function GlobalStockIndexesReadable() {
  return rpc('GlobalStockIndexesReadable');
}

export function Greet(arg1) {
  return rpc('Greet', arg1);
}

export function HotEvent(arg1) {
  return rpc('HotEvent', arg1);
}

export function HotStock(arg1) {
  return rpc('HotStock', arg1);
}

export function HotTopic(arg1) {
  return rpc('HotTopic', arg1);
}

export function ImportSkillFromBase64(arg1) {
  return rpc('ImportSkillFromBase64', arg1);
}

export function IndustryDetail(arg1) {
  return rpc('IndustryDetail', arg1);
}

export function IndustryKLine(arg1) {
  return rpc('IndustryKLine', arg1);
}

export function IndustryRealHead(arg1) {
  return rpc('IndustryRealHead', arg1);
}

export function IndustryResearchReport(arg1) {
  return rpc('IndustryResearchReport', arg1);
}

export function InitCronTasks() {
  return rpc('InitCronTasks');
}

export function InitializeGroupSort() {
  return rpc('InitializeGroupSort');
}

export function InvestCalendarTimeLine(arg1) {
  return rpc('InvestCalendarTimeLine', arg1);
}

export function IsHKTradingTime() {
  return rpc('IsHKTradingTime');
}

export function IsTradingDay(arg1) {
  return rpc('IsTradingDay', arg1);
}

export function IsTradingTime() {
  return rpc('IsTradingTime');
}

export function IsUSTradingTime() {
  return rpc('IsUSTradingTime');
}

export function ListAIServicesForKB() {
  return rpc('ListAIServicesForKB');
}

export function ListAgentFeedback(arg1, arg2) {
  return rpc('ListAgentFeedback', arg1, arg2);
}

export function ListFilesystemSkills() {
  return rpc('ListFilesystemSkills');
}

export function ListKBDocuments(arg1) {
  return rpc('ListKBDocuments', arg1);
}

export function ListKBDocumentsPaged(arg1, arg2, arg3) {
  return rpc('ListKBDocumentsPaged', arg1, arg2, arg3);
}

export function ListKnowledgeBases() {
  return rpc('ListKnowledgeBases');
}

export function ListRecommendBacktest(arg1, arg2) {
  return rpc('ListRecommendBacktest', arg1, arg2);
}

export function ListRecommendBacktestByPrompt(arg1, arg2, arg3, arg4) {
  return rpc('ListRecommendBacktestByPrompt', arg1, arg2, arg3, arg4);
}

export function ListSkillFiles(arg1) {
  return rpc('ListSkillFiles', arg1);
}

export function LongTigerRank(arg1) {
  return rpc('LongTigerRank', arg1);
}

export function NewChatStream(arg1, arg2, arg3, arg4, arg5, arg6, arg7) {
  return rpc('NewChatStream', arg1, arg2, arg3, arg4, arg5, arg6, arg7);
}

export function NewsPush(arg1) {
  return rpc('NewsPush', arg1);
}

export function PackSkillToBase64(arg1) {
  return rpc('PackSkillToBase64', arg1);
}

export function PromptPlazaRequest(arg1, arg2, arg3, arg4, arg5, arg6) {
  return rpc('PromptPlazaRequest', arg1, arg2, arg3, arg4, arg5, arg6);
}

export function ReFleshTelegraphList(arg1) {
  return rpc('ReFleshTelegraphList', arg1);
}

export function ReadSkillFile(arg1, arg2) {
  return rpc('ReadSkillFile', arg1, arg2);
}

export function RefreshAllTdxTransactionData(arg1) {
  return rpc('RefreshAllTdxTransactionData', arg1);
}

export function RefreshHistoryTdxTransactionData(arg1, arg2) {
  return rpc('RefreshHistoryTdxTransactionData', arg1, arg2);
}

export function RefreshHotMoneySeats(arg1) {
  return rpc('RefreshHotMoneySeats', arg1);
}

export function RelearnUserProfile() {
  return rpc('RelearnUserProfile');
}

export function RemoveConcept(arg1) {
  return rpc('RemoveConcept', arg1);
}

export function RemoveGroup(arg1) {
  return rpc('RemoveGroup', arg1);
}

export function RemoveStockConcept(arg1, arg2, arg3) {
  return rpc('RemoveStockConcept', arg1, arg2, arg3);
}

export function RemoveStockGroup(arg1, arg2, arg3) {
  return rpc('RemoveStockGroup', arg1, arg2, arg3);
}

export function ResetHotMoneySeats() {
  return rpc('ResetHotMoneySeats');
}

export function RunRecommendBacktest(arg1) {
  return rpc('RunRecommendBacktest', arg1);
}

export function RzrqRank(arg1, arg2, arg3, arg4, arg5, arg6) {
  return rpc('RzrqRank', arg1, arg2, arg3, arg4, arg5, arg6);
}

export function RzrqTrend(arg1, arg2) {
  return rpc('RzrqTrend', arg1, arg2);
}

export function SaveAIResponseResult(arg1, arg2, arg3, arg4, arg5, arg6) {
  return rpc('SaveAIResponseResult', arg1, arg2, arg3, arg4, arg5, arg6);
}

export function SaveAiAssistantSession(arg1, arg2) {
  return rpc('SaveAiAssistantSession', arg1, arg2);
}

export function SaveAsMarkdown(arg1, arg2) {
  return rpc('SaveAsMarkdown', arg1, arg2);
}

export function SaveCustomStrategy(arg1) {
  return rpc('SaveCustomStrategy', arg1);
}

export function SaveDailyOperationPlan(arg1) {
  return rpc('SaveDailyOperationPlan', arg1);
}

export function SaveHotMoneySeats(arg1) {
  return rpc('SaveHotMoneySeats', arg1);
}

export function SaveImage(arg1, arg2) {
  return rpc('SaveImage', arg1, arg2);
}

export function SaveKeyDepartments(arg1) {
  return rpc('SaveKeyDepartments', arg1);
}

export function SaveStockChangesToHistory(arg1) {
  return rpc('SaveStockChangesToHistory', arg1);
}

export function SaveUserProfile(arg1) {
  return rpc('SaveUserProfile', arg1);
}

export function SaveWordFile(arg1, arg2) {
  return rpc('SaveWordFile', arg1, arg2);
}

export function SearchAllKnowledge(arg1, arg2) {
  return rpc('SearchAllKnowledge', arg1, arg2);
}

export function SearchCronTasks(arg1) {
  return rpc('SearchCronTasks', arg1);
}

export function SearchFundCodes(arg1) {
  return rpc('SearchFundCodes', arg1);
}

export function SearchKnowledgeBase(arg1, arg2, arg3) {
  return rpc('SearchKnowledgeBase', arg1, arg2, arg3);
}

export function SearchLongTermMemory(arg1, arg2) {
  return rpc('SearchLongTermMemory', arg1, arg2);
}

export function SearchStock(arg1) {
  return rpc('SearchStock', arg1);
}

export function SendDingDingMessage(arg1, arg2) {
  return rpc('SendDingDingMessage', arg1, arg2);
}

export function SendDingDingMessageByType(arg1, arg2, arg3) {
  return rpc('SendDingDingMessageByType', arg1, arg2, arg3);
}

export function SendFeishuMessage(arg1, arg2) {
  return rpc('SendFeishuMessage', arg1, arg2);
}

export function SendFeishuMessageByType(arg1, arg2, arg3) {
  return rpc('SendFeishuMessageByType', arg1, arg2, arg3);
}

export function SetAlarmChangePercent(arg1, arg2, arg3) {
  return rpc('SetAlarmChangePercent', arg1, arg2, arg3);
}

export function SetCostPriceAndVolume(arg1, arg2, arg3) {
  return rpc('SetCostPriceAndVolume', arg1, arg2, arg3);
}

export function SetLongTermMemoryAiConfigId(arg1) {
  return rpc('SetLongTermMemoryAiConfigId', arg1);
}

export function SetProfileLearnAiConfigId(arg1) {
  return rpc('SetProfileLearnAiConfigId', arg1);
}

export function SetStockAICron(arg1, arg2) {
  return rpc('SetStockAICron', arg1, arg2);
}

export function SetStockSort(arg1, arg2) {
  return rpc('SetStockSort', arg1, arg2);
}

export function SetTradingPrice(arg1, arg2, arg3, arg4, arg5) {
  return rpc('SetTradingPrice', arg1, arg2, arg3, arg4, arg5);
}

export function SetUserProfileEnabled(arg1) {
  return rpc('SetUserProfileEnabled', arg1);
}

export function ShareAnalysis(arg1, arg2) {
  return rpc('ShareAnalysis', arg1, arg2);
}

export function ShareText(arg1, arg2) {
  return rpc('ShareText', arg1, arg2);
}

export function StartFeishuBot() {
  return rpc('StartFeishuBot');
}

export function StartMCPOAuth(arg1) {
  return rpc('StartMCPOAuth', arg1);
}

export function StockNotice(arg1) {
  return rpc('StockNotice', arg1);
}

export function StockResearchReport(arg1) {
  return rpc('StockResearchReport', arg1);
}

export function StopFeishuBot() {
  return rpc('StopFeishuBot');
}

export function SubmitAgentFeedback(arg1) {
  return rpc('SubmitAgentFeedback', arg1);
}

export function SummaryStockNews(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8) {
  return rpc('SummaryStockNews', arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
}

export function TestMCPServer(arg1) {
  return rpc('TestMCPServer', arg1);
}

export function UnFollow(arg1) {
  return rpc('UnFollow', arg1);
}

export function UnFollowFund(arg1) {
  return rpc('UnFollowFund', arg1);
}

export function UpdateAiConfigs(arg1) {
  return rpc('UpdateAiConfigs', arg1);
}

export function UpdateAiRecommendStocksAlert(arg1, arg2) {
  return rpc('UpdateAiRecommendStocksAlert', arg1, arg2);
}

export function UpdateConcept(arg1, arg2) {
  return rpc('UpdateConcept', arg1, arg2);
}

export function UpdateConfig(arg1) {
  return rpc('UpdateConfig', arg1);
}

export function UpdateCronTask(arg1) {
  return rpc('UpdateCronTask', arg1);
}

export function UpdateDailyOperationPlanAlert(arg1, arg2) {
  return rpc('UpdateDailyOperationPlanAlert', arg1, arg2);
}

export function UpdateDailyOperationPlanStatus(arg1, arg2) {
  return rpc('UpdateDailyOperationPlanStatus', arg1, arg2);
}

export function UpdateFilesystemSkillDescription(arg1, arg2) {
  return rpc('UpdateFilesystemSkillDescription', arg1, arg2);
}

export function UpdateGroup(arg1, arg2) {
  return rpc('UpdateGroup', arg1, arg2);
}

export function UpdateGroupSort(arg1, arg2) {
  return rpc('UpdateGroupSort', arg1, arg2);
}

export function UpdateMCPServer(arg1) {
  return rpc('UpdateMCPServer', arg1);
}

export function UpdatePromptTemplate(arg1) {
  return rpc('UpdatePromptTemplate', arg1);
}

export function UpdateSkill(arg1) {
  return rpc('UpdateSkill', arg1);
}

export function UpdateTradingRecord(arg1) {
  return rpc('UpdateTradingRecord', arg1);
}

export function UploadImageToImageBed(arg1, arg2) {
  return rpc('UploadImageToImageBed', arg1, arg2);
}

export function UploadKBFile(arg1, arg2) {
  return rpc('UploadKBFile', arg1, arg2);
}

export function UploadKBFiles(arg1, arg2) {
  return rpc('UploadKBFiles', arg1, arg2);
}

export function ValidateCronExpr(arg1) {
  return rpc('ValidateCronExpr', arg1);
}

export function WriteSkillFile(arg1, arg2, arg3) {
  return rpc('WriteSkillFile', arg1, arg2, arg3);
}
