const MODEL_RESOLUTIONS = {
  "seedance2.5-480p-token-no-video": "480p",
  "seedance2.5-480p-token-with-video": "480p",
  "seedance2.5-720p-token-no-video": "720p",
  "seedance2.5-720p-token-with-video": "720p",
  "seedance2.5-1080p-token-no-video": "1080p",
  "seedance2.5-1080p-token-with-video": "1080p",
  "seedance2-mini-480p-token-no-video": "480p",
  "seedance2-mini-480p-token-with-video": "480p",
  "seedance2-mini-720p-token-no-video": "720p",
  "seedance2-mini-720p-token-with-video": "720p",
  "seedance2-fast-480p-token-no-video": "480p",
  "seedance2-fast-480p-token-with-video": "480p",
  "seedance2-fast-720p-token-no-video": "720p",
  "seedance2-fast-720p-token-with-video": "720p",
  "seedance2-4k-token-no-video": "4k",
  "seedance2-4k-token-with-video": "4k",
  "seedance2-480p-token-no-video": "480p",
  "seedance2-480p-token-with-video": "480p",
  "seedance2-720p-token-no-video": "720p",
  "seedance2-720p-token-with-video": "720p",
  "seedance2-1080p-token-no-video": "1080p",
  "seedance2-1080p-token-with-video": "1080p",
};

const RESOLUTIONS = ["480p", "720p", "1080p", "4k"];
const ASPECT_RATIOS = ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "adaptive"];
// The supplier manual gives explicit size aliases for 720p only. Other tiers
// are selected by the model ID, so do not infer arbitrary pixel mappings.
const SIZE_RESOLUTIONS = { "1280x720": "720p", "720x1280": "720p" };
const VIDEO_USAGE_SCHEMA = {
  tokens: {
    type: "number",
    unit: "token",
    description: { en: "Video generation token unit price", zh: "视频生成 Token 单价" },
  },
  resolution: {
    enum: RESOLUTIONS,
    description: { en: "Output video resolution", zh: "输出视频分辨率" },
  },
  video_input: {
    enum: ["none", "video"],
    enumLabels: {
      none: { en: "No reference video", zh: "无参考视频" },
      video: { en: "With reference video", zh: "有参考视频" },
    },
    description: { en: "Reference video input", zh: "参考视频输入" },
  },
};

function videoUsageExamples(resolution, videoInput) {
  const inputLabel = videoInput === "video" ? "reference video" : "no reference video";
  return [
    {
      label: "100k tokens · " + resolution + " · " + inputLabel,
      facts: { tokens: 100000, resolution: resolution, video_input: videoInput },
    },
  ];
}

function usageProfiles() {
  const profiles = [];
  for (const resolution of RESOLUTIONS) {
    for (const videoInput of ["none", "video"]) {
      const models = Object.keys(MODEL_RESOLUTIONS).filter(function (model) {
        return MODEL_RESOLUTIONS[model] === resolution && model.endsWith(videoInput === "video" ? "-with-video" : "-no-video");
      });
      profiles.push({
        models: models,
        schema: Object.assign({}, VIDEO_USAGE_SCHEMA, {
          resolution: {
            enum: [resolution],
            description: VIDEO_USAGE_SCHEMA.resolution.description,
          },
          video_input: {
            enum: [videoInput],
            description: VIDEO_USAGE_SCHEMA.video_input.description,
          },
        }),
        examples: videoUsageExamples(resolution, videoInput),
      });
    }
  }
  return profiles;
}

export const meta = {
  apiVersion: 1,
  key: "supplyvideo",
  name: "Supply Video",
  description: {
    en: "Seedance video generation through a supplier gateway",
    zh: "通过供应商网关生成 Seedance 视频",
  },
  version: "1.0.0",
  author: { name: "QuantumNous" },
  models: Object.keys(MODEL_RESOLUTIONS),
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  usageSchema: VIDEO_USAGE_SCHEMA,
  usageExamples: videoUsageExamples("480p", "none"),
  usageProfiles: usageProfiles(),
  protocols: ["openai_video"],
};

function objectValue(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function modelProfile(model) {
  if (typeof model !== "string" || !Object.prototype.hasOwnProperty.call(MODEL_RESOLUTIONS, model)) throw new Error("model is not supported by Supply Video");
  const seedance25 = model.startsWith("seedance2.5-");
  return {
    resolution: MODEL_RESOLUTIONS[model],
    withVideo: model.endsWith("-with-video"),
    maxImages: seedance25 ? 30 : 9,
    maxVideos: seedance25 ? 10 : 3,
    minSeconds: 4,
    maxSeconds: seedance25 ? 30 : 15,
  };
}

function profileModelFor(declaredModel, upstreamModel) {
  return typeof upstreamModel === "string" && Object.prototype.hasOwnProperty.call(MODEL_RESOLUTIONS, upstreamModel) ? upstreamModel : declaredModel;
}

function unwrapData(body) {
  if (!objectValue(body)) return null;
  return objectValue(body.data) ? body.data : body;
}

function optionalURL(value, field) {
  if (typeof value !== "string" || !value.trim() || value.trim() !== value || !/^https?:\/\/[^\s/?#]+(?:[/?#][^\s]*)?$/i.test(value))
    throw new Error(field + ".url must be an HTTP or HTTPS URL");
  return value;
}

function roleItems(value, field, allowedRoles, maxCount) {
  if (value === undefined) return [];
  if (!Array.isArray(value)) throw new Error(field + " must be an array");
  if (value.length > maxCount) throw new Error(field + " supports at most " + maxCount + " items");
  const seen = new Set();
  const counts = {};
  const items = [];
  for (const item of value) {
    if (!objectValue(item)) throw new Error(field + " items must be objects");
    for (const key of Object.keys(item)) {
      if (key !== "url" && key !== "role") throw new Error(field + " items may contain only url and role");
    }
    if (typeof item.role !== "string" || !allowedRoles.includes(item.role)) throw new Error(field + ".role is not supported");
    const url = optionalURL(item.url, field);
    if (seen.has(url)) throw new Error(field + " contains a duplicate URL");
    seen.add(url);
    counts[item.role] = (counts[item.role] || 0) + 1;
    if (["first_frame", "last_frame"].includes(item.role) && counts[item.role] > 1) throw new Error("image_with_roles may contain only one " + item.role);
    items.push({ url: url, role: item.role });
  }
  if (counts.last_frame && !counts.first_frame) throw new Error("last_frame requires first_frame");
  return items;
}

function normalizeRequest(request, declaredModel, profileModel) {
  if (!objectValue(request)) throw new Error("request body must be an object");
  const profileName = profileModel || declaredModel;
  const profile = modelProfile(profileName);
  const allowedKeys = new Set([
    "model",
    "prompt",
    "seconds",
    "duration",
    "aspect_ratio",
    "resolution",
    "size",
    "video_operation",
    "image_with_roles",
    "video_with_roles",
    "audio_with_roles",
  ]);
  for (const key of Object.keys(request)) {
    if (!allowedKeys.has(key)) throw new Error("field " + key + " is not supported");
  }
  if (request.model !== undefined && request.model !== declaredModel) throw new Error("model must match the selected model");
  if (request.video_operation !== undefined && request.video_operation !== "generate") throw new Error("only video_operation generate is supported");
  if (request.audio_with_roles !== undefined) throw new Error("audio_with_roles is not supported in phase 1");
  if (request.resolution !== undefined && request.resolution !== profile.resolution) throw new Error("resolution conflicts with the selected model");
  if (request.size !== undefined) {
    if (typeof request.size !== "string" || !Object.prototype.hasOwnProperty.call(SIZE_RESOLUTIONS, request.size))
      throw new Error("size must be one of " + Object.keys(SIZE_RESOLUTIONS).join(", "));
    if (SIZE_RESOLUTIONS[request.size] !== profile.resolution) throw new Error("size conflicts with the selected model");
  }
  if (typeof request.prompt !== "string" || !request.prompt.trim()) throw new Error("field prompt is required");

  const seconds = request.seconds;
  if (request.duration !== undefined) {
    if (seconds === undefined || request.duration !== seconds) throw new Error("duration must match seconds");
  }
  if (!Number.isInteger(seconds) || seconds < profile.minSeconds || seconds > profile.maxSeconds)
    throw new Error("seconds must be an integer between " + profile.minSeconds + " and " + profile.maxSeconds);

  const aspectRatio = request.aspect_ratio === undefined ? "16:9" : request.aspect_ratio;
  if (typeof aspectRatio !== "string" || !ASPECT_RATIOS.includes(aspectRatio)) throw new Error("aspect_ratio must be one of " + ASPECT_RATIOS.join(", "));

  const images = roleItems(request.image_with_roles, "image_with_roles", ["first_frame", "last_frame", "reference_image"], profile.maxImages);
  const firstFrames = images.filter((item) => item.role === "first_frame").length;
  const lastFrames = images.filter((item) => item.role === "last_frame").length;
  const referenceImages = images.filter((item) => item.role === "reference_image").length;
  const frameMode = firstFrames > 0 || lastFrames > 0;
  if (frameMode) {
    if (firstFrames !== 1) throw new Error("image_with_roles must contain exactly one first_frame");
    if (referenceImages > 0) throw new Error("frame pair cannot be combined with reference_image");
  }
  const videos = roleItems(request.video_with_roles, "video_with_roles", ["reference_video"], profile.maxVideos);
  if (profile.withVideo && videos.length === 0) throw new Error("with-video models require at least one reference_video");
  if (!profile.withVideo && videos.length > 0) throw new Error("no-video models do not accept reference_video");
  if (frameMode && videos.length > 0) throw new Error("frame pair cannot be combined with reference_video");
  // The supplier manual requires adaptive aspect ratio whenever Seedance 2.5
  // uses a last frame; an explicit ratio would change that mode's contract.
  if (lastFrames > 0 && profileName.startsWith("seedance2.5-") && aspectRatio !== "adaptive")
    throw new Error("seedance2.5 last_frame requires aspect_ratio adaptive");

  const body = {
    model: declaredModel,
    prompt: request.prompt,
    seconds: seconds,
    aspect_ratio: aspectRatio,
  };
  if (request.video_operation !== undefined) body.video_operation = "generate";
  if (request.size !== undefined) body.size = request.size;
  if (request.image_with_roles !== undefined) body.image_with_roles = images;
  if (request.video_with_roles !== undefined) body.video_with_roles = videos;
  return { body: body, profile: profile, videoInput: videos.length > 0 ? "video" : "none" };
}

function statusResult(body) {
  const data = unwrapData(body);
  if (!data || typeof data.status !== "string" || !data.status.trim()) return { data: data, status: "UNKNOWN" };
  const status = data.status.trim().toLowerCase();
  const statuses = {
    queued: "QUEUED",
    in_progress: "IN_PROGRESS",
    completed: "SUCCESS",
    failed: "FAILURE",
  };
  return { data: data, status: statuses[status] || "UNKNOWN", rawStatus: status };
}

function validTotalTokens(body) {
  const data = unwrapData(body);
  const usage = data && data.usage;
  const tokens = usage && usage.total_tokens;
  return typeof tokens === "number" && Number.isFinite(tokens) && tokens > 0 ? tokens : null;
}

function taskIdFrom(body) {
  const data = unwrapData(body);
  const taskId = data && (data.id !== undefined ? data.id : data.task_id);
  if (typeof taskId !== "string" || !taskId.trim()) throw new Error("upstream task id is missing or invalid");
  return taskId;
}

function videoURL(ctx, taskId, suffix) {
  if (typeof ctx.baseUrl !== "string" || !ctx.baseUrl) throw new Error("upstream base URL is required");
  return ctx.baseUrl + "/v1/videos/" + encodeURIComponent(taskId) + (suffix || "");
}

export function buildSubmitRequest(ctx) {
  const upstreamModel = ctx.upstreamModel || ctx.model;
  const normalized = normalizeRequest(ctx.requestBody, ctx.model, profileModelFor(ctx.model, upstreamModel));
  return {
    url: ctx.baseUrl + "/v1/videos",
    method: "POST",
    headers: { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json" },
    body: Object.assign({}, normalized.body, { model: upstreamModel }),
  };
}

export function parseSubmitResponse(_ctx, response) {
  const body = response && response.body;
  const data = unwrapData(body);
  if (!data) throw new Error("invalid video submission response");
  const taskId = taskIdFrom(body);
  const parsedStatus = statusResult(body);
  if (parsedStatus.status === "UNKNOWN" && parsedStatus.rawStatus) throw new Error("unrecognized task status: " + parsedStatus.rawStatus);
  const result = { taskId: taskId, taskData: body };
  if (parsedStatus.status === "SUCCESS") {
    const immediate = { status: "SUCCESS", progress: "100%" };
    const tokens = validTotalTokens(body);
    if (tokens !== null) immediate.totalTokens = tokens;
    result.immediate = immediate;
  } else if (parsedStatus.status === "FAILURE") {
    const error = parsedStatus.data.error;
    result.immediate = {
      status: "FAILURE",
      progress: "100%",
      reason: error && typeof error.message === "string" ? error.message : "video generation failed",
    };
  }
  return result;
}

export function extractUsage(ctx) {
  const upstreamModel = ctx.upstreamModel || ctx.model;
  const normalized = normalizeRequest(ctx.requestBody, ctx.model, profileModelFor(ctx.model, upstreamModel));
  return {
    tokens: 0,
    resolution: normalized.profile.resolution,
    video_input: normalized.videoInput,
  };
}

export function buildQueryRequest(ctx) {
  if (typeof ctx.taskId !== "string" || !ctx.taskId.trim()) throw new Error("upstream task id is required");
  return { url: videoURL(ctx, ctx.taskId), method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey } };
}

export function parseTaskResult(_ctx, body) {
  const parsed = statusResult(body);
  const result = { status: parsed.status };
  if (parsed.status === "UNKNOWN") {
    result.reason = "unrecognized task status: " + String(parsed.rawStatus || "missing");
    return result;
  }
  if (parsed.status === "SUCCESS") {
    result.progress = "100%";
    const tokens = validTotalTokens(body);
    if (tokens !== null) result.totalTokens = tokens;
    return result;
  }
  if (parsed.status === "FAILURE") {
    const error = parsed.data && parsed.data.error;
    result.reason = error && typeof error.message === "string" ? error.message : "video generation failed";
    return result;
  }
  const progress = parsed.data && parsed.data.progress;
  if (typeof progress === "number" && Number.isFinite(progress) && progress >= 0 && progress <= 100) result.progress = progress + "%";
  return result;
}

export function extractUsageOnComplete(_ctx, taskResult, body) {
  if (!taskResult || taskResult.status !== "SUCCESS") return {};
  const tokens = validTotalTokens(body);
  if (tokens === null) {
    console.log("warning: completed video response has no valid usage.total_tokens; actual token usage was not reported");
    return {};
  }
  return { tokens: tokens, totalTokens: tokens };
}

export function listArtifacts(task) {
  return task && task.status === "SUCCESS" ? [{ key: "video", type: "video" }] : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  if (ctx.clientRequest.method !== "GET" && ctx.clientRequest.method !== "HEAD") throw new Error("video content supports GET and HEAD only");
  if (typeof ctx.upstreamTaskId !== "string" || !ctx.upstreamTaskId.trim()) throw new Error("upstream task id is required");
  return {
    url: videoURL(ctx, ctx.upstreamTaskId, "/content"),
    method: ctx.clientRequest.method,
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

export const protocols = {
  openai_video: {
    decodeRequest(ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const declaredModel = ctx.model;
      const normalized = normalizeRequest(ctx.body.value, declaredModel, profileModelFor(declaredModel, ctx.upstreamModel || declaredModel));
      const hasMedia = normalized.body.image_with_roles || normalized.body.video_with_roles;
      return {
        kind: "submit",
        model: declaredModel,
        action: hasMedia ? "image_to_video" : "text_to_video",
        requestBody: normalized.body,
      };
    },
    render(_ctx, task) {
      const data = unwrapData(task.data) || {};
      const statuses = {
        NOT_START: "queued",
        SUBMITTED: "queued",
        QUEUED: "queued",
        IN_PROGRESS: "in_progress",
        SUCCESS: "completed",
        FAILURE: "failed",
      };
      const output = {
        id: task.task_id,
        object: "video",
        model: task.properties ? task.properties.origin_model_name || "" : "",
        status: statuses[task.status] || "unknown",
        progress: Number(String(task.progress || "0").replace("%", "")),
        created_at: Number(task.created_at || 0),
      };
      if (task.status === "FAILURE") {
        output.error = {
          code: "video_generation_failed",
          message: typeof task.fail_reason === "string" && task.fail_reason ? task.fail_reason : "video generation failed",
        };
      }
      // The public representation must never expose the supplier's raw task
      // object: it can carry the private upstream task id and signed media URLs.
      // Only the measured total token count is safe to publish.
      const usage = data.usage;
      if (objectValue(usage)) {
        const total = usage.total_tokens;
        if (typeof total === "number" && Number.isFinite(total) && total >= 0) output.usage = { total_tokens: total };
      }
      return output;
    },
  },
};
