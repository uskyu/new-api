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

// TA's non-Seedance video group bills by actual output/input seconds. The
// supplier returns these two usage facts for every model in this group.
const SECOND_USAGE_SCHEMA = {
  output_seconds: {
    type: "number",
    unit: "second",
    description: { en: "Output video unit price", zh: "输出视频单价" },
  },
  input_seconds: {
    type: "number",
    unit: "second",
    description: { en: "Input video unit price", zh: "输入视频单价" },
  },
};

// Family-level request contract shared by the per-second models. Resolution is
// locked by the model ID, exactly as for the token models above.
const SECOND_FAMILIES = {
  happyhorse: {
    minSeconds: 3,
    maxSeconds: 15,
    autoSeconds: false,
    aspects: ["16:9", "9:16", "1:1", "4:3", "3:4"],
    defaultAspect: "16:9",
    maxImages: 9,
    maxVideos: 0,
    maxAudios: 0,
    imageRoles: ["first_frame", "reference_image"],
    videoMode: "forbidden",
    framePair: false,
    framesExclusiveWithVideo: false,
    audioRequiresMedia: false,
    adaptiveWithFrames: false,
    adaptiveWithoutMediaBlocked: false,
  },
  grok: {
    minSeconds: 1,
    maxSeconds: 15,
    autoSeconds: false,
    aspects: ["1:1", "16:9", "9:16", "3:2", "2:3"],
    defaultAspect: "16:9",
    maxImages: 7,
    maxVideos: 0,
    maxAudios: 0,
    imageRoles: ["first_frame", "reference_image"],
    videoMode: "forbidden",
    framePair: false,
    framesExclusiveWithVideo: false,
    audioRequiresMedia: false,
    adaptiveWithFrames: false,
    adaptiveWithoutMediaBlocked: false,
  },
  wan: {
    minSeconds: 2,
    maxSeconds: 30,
    autoSeconds: true,
    aspects: ["adaptive", "16:9", "9:16", "1:1", "4:3", "3:4"],
    defaultAspect: "16:9",
    maxImages: 10,
    maxVideos: 5,
    maxAudios: 5,
    imageRoles: ["first_frame", "last_frame", "reference_image"],
    videoMode: "optional",
    framePair: true,
    framesExclusiveWithVideo: true,
    audioRequiresMedia: true,
    adaptiveWithFrames: false,
    adaptiveWithoutMediaBlocked: false,
  },
  kling: {
    minSeconds: 3,
    maxSeconds: 30,
    autoSeconds: false,
    aspects: ["auto", "16:9", "9:16", "1:1", "21:9"],
    defaultAspect: "auto",
    maxImages: 10,
    maxVideos: 5,
    maxAudios: 0,
    imageRoles: ["first_frame", "last_frame", "reference_image"],
    videoMode: "by-model",
    framePair: true,
    framesExclusiveWithVideo: true,
    audioRequiresMedia: false,
    adaptiveWithFrames: false,
    adaptiveWithoutMediaBlocked: false,
  },
  minimax: {
    minSeconds: 5,
    maxSeconds: 15,
    autoSeconds: false,
    aspects: ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "adaptive"],
    defaultAspect: "16:9",
    maxImages: 9,
    maxVideos: 3,
    maxAudios: 3,
    maxTotalMaterials: 12,
    imageRoles: ["first_frame", "last_frame", "reference_image"],
    videoMode: "optional",
    framePair: true,
    framesExclusiveWithVideo: false,
    audioRequiresMedia: true,
    adaptiveWithFrames: true,
    adaptiveWithoutMediaBlocked: true,
  },
};

// Exact model ID -> [family, locked resolution]. Keep this list aligned with
// the supplier's TA-其他视频模型分组 pricing records.
const SECOND_MODELS = {
  "happyhorse1.1-720p-second": ["happyhorse", "720p"],
  "happyhorse1.1-1080p-second": ["happyhorse", "1080p"],
  "happyhorse1.1-720p-edit-second": ["happyhorse", "720p"],
  "grok-video1.5-480p-second": ["grok", "480p"],
  "grok-video1.5-720p-second": ["grok", "720p"],
  "wan3.0-video-480p": ["wan", "480p"],
  "wan3.0-video-720p": ["wan", "720p"],
  "wan3.0-video-1080p": ["wan", "1080p"],
  "wan3.0-video-prime-480p": ["wan", "480p"],
  "wan3.0-video-prime-720p": ["wan", "720p"],
  "wan3.0-video-prime-1080p": ["wan", "1080p"],
  "kling4-720p-second-no-video": ["kling", "720p"],
  "kling4-720p-second-with-video": ["kling", "720p"],
  "kling4-1080p-second-no-video": ["kling", "1080p"],
  "kling4-1080p-second-with-video": ["kling", "1080p"],
  "minimax-h3-max-480p-second": ["minimax", "480p"],
  "minimax-h3-max-768p-second": ["minimax", "768p"],
  "minimax-h3-max-1080p-second": ["minimax", "1080p"],
};

// PAI's Seedance group bills either per successful task or per billed second.
// Its published duration ranges are used where confirmed; the seven records
// the supplier marked "待确认" keep only a sanity bound and let the upstream
// reject an out-of-contract duration.
const PAI_MODELS = {
  "seedance2-fast-720p-once-1": { kind: "call", minSeconds: 4, maxSeconds: 15 },
  "seedance2-fast-720p-once-2": { kind: "call", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-480p-once": { kind: "call", minSeconds: 4, maxSeconds: 15 },
  "seedance2-1080p-once": { kind: "call", minSeconds: 4, maxSeconds: 15 },
  "seedance2-720p-once-1": { kind: "call", minSeconds: 4, maxSeconds: 15 },
  "seedance2-720p-once-2": { kind: "call", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-720p-once-3": { kind: "call", minSeconds: 4, maxSeconds: 15 },
  "seedance2.5-720p-once": { kind: "call", minSeconds: 5, maxSeconds: 30 },
  "seedance2-mini-480p-second": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2-mini-720p-second": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2-fast-480p-second-1": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2-fast-480p-second-2": { kind: "second", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-fast-720p-second-1": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2-fast-720p-second-2": { kind: "second", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-480p-second-1": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2-480p-second-2": { kind: "second", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-720p-second-1": { kind: "second", minSeconds: 5, maxSeconds: 15 },
  "seedance2-720p-second-2": { kind: "second", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-1080p-second-1": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2-1080p-second-2": { kind: "second", minSeconds: 1, maxSeconds: 60, rangeUnconfirmed: true },
  "seedance2-4k-second": { kind: "second", minSeconds: 4, maxSeconds: 15 },
  "seedance2.5-480p-second-1": { kind: "second", minSeconds: 4, maxSeconds: 30 },
  "seedance2.5-480p-second-2": { kind: "second", minSeconds: 4, maxSeconds: 30 },
  "seedance2.5-480p-second-3": { kind: "second", minSeconds: 4, maxSeconds: 30 },
  "seedance2.5-480p-second-4": { kind: "second", minSeconds: 4, maxSeconds: 30 },
  "seedance2.5-720p-second-1": { kind: "second", minSeconds: 4, maxSeconds: 30 },
  "seedance2.5-720p-second-2": { kind: "second", minSeconds: 4, maxSeconds: 30 },
  "seedance2.5-720p-second-3": { kind: "second", minSeconds: 4, maxSeconds: 30 },
};

const PAI_CALL_MODELS = Object.keys(PAI_MODELS).filter((model) => PAI_MODELS[model].kind === "call");
const PAI_SECOND_MODELS = Object.keys(PAI_MODELS).filter((model) => PAI_MODELS[model].kind === "second");

const REQUEST_FIELDS = [
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
];
const CONTENT_USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36";
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
  profiles.push({
    models: Object.keys(SECOND_MODELS),
    schema: SECOND_USAGE_SCHEMA,
    examples: [
      {
        label: "5s output · no input video",
        facts: { output_seconds: 5, input_seconds: 0 },
      },
    ],
  });
  profiles.push({
    models: PAI_CALL_MODELS,
    schema: {},
  });
  profiles.push({
    models: PAI_SECOND_MODELS,
    schema: SECOND_USAGE_SCHEMA,
    examples: [
      {
        label: "4s output · no input video",
        facts: { output_seconds: 4, input_seconds: 0 },
      },
    ],
  });
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
  version: "1.2.4",
  author: { name: "QuantumNous" },
  models: Object.keys(MODEL_RESOLUTIONS).concat(Object.keys(SECOND_MODELS), PAI_CALL_MODELS, PAI_SECOND_MODELS),
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  allowedHosts: ["cdn.paisio.online", "files.toapis.xyz", "ark-acg-cn-beijing.tos-cn-beijing.volces.com"],
  usageSchema: VIDEO_USAGE_SCHEMA,
  usageExamples: videoUsageExamples("480p", "none"),
  usageProfiles: usageProfiles(),
  protocols: ["openai_video"],
};

function objectValue(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isKnownModel(model) {
  return (
    typeof model === "string" &&
    (Object.prototype.hasOwnProperty.call(MODEL_RESOLUTIONS, model) ||
      Object.prototype.hasOwnProperty.call(SECOND_MODELS, model) ||
      Object.prototype.hasOwnProperty.call(PAI_MODELS, model))
  );
}

function modelProfile(model) {
  if (typeof model === "string" && Object.prototype.hasOwnProperty.call(PAI_MODELS, model)) {
    const record = PAI_MODELS[model];
    return {
      billing: record.kind === "call" ? "pai_call" : "pai_second",
      family: "pai",
      minSeconds: record.minSeconds,
      maxSeconds: record.maxSeconds,
      rangeUnconfirmed: record.rangeUnconfirmed === true,
    };
  }
  if (typeof model === "string" && Object.prototype.hasOwnProperty.call(SECOND_MODELS, model)) {
    const entry = SECOND_MODELS[model];
    const family = SECOND_FAMILIES[entry[0]];
    const withVideo = model.endsWith("-with-video");
    const videoMode = family.videoMode === "by-model" ? (withVideo ? "required" : "forbidden") : family.videoMode;
    return Object.assign({}, family, {
      billing: "second",
      family: entry[0],
      resolution: entry[1],
      withVideo: withVideo,
      videoMode: videoMode,
    });
  }
  if (typeof model !== "string" || !Object.prototype.hasOwnProperty.call(MODEL_RESOLUTIONS, model)) throw new Error("model is not supported by Supply Video");
  const seedance25 = model.startsWith("seedance2.5-");
  return {
    billing: "token",
    resolution: MODEL_RESOLUTIONS[model],
    withVideo: model.endsWith("-with-video"),
    maxImages: seedance25 ? 30 : 9,
    maxVideos: seedance25 ? 10 : 3,
    minSeconds: 4,
    maxSeconds: seedance25 ? 30 : 15,
  };
}

function profileModelFor(declaredModel, upstreamModel) {
  return isKnownModel(upstreamModel) ? upstreamModel : declaredModel;
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
    if (["first_frame", "last_frame"].includes(item.role) && counts[item.role] > 1) throw new Error(field + " may contain only one " + item.role);
    items.push({ url: url, role: item.role });
  }
  if (counts.last_frame && !counts.first_frame) throw new Error("last_frame requires first_frame");
  return items;
}

function normalizeSecondRequest(request, declaredModel, profile) {
  for (const key of Object.keys(request)) {
    if (!REQUEST_FIELDS.includes(key)) throw new Error("field " + key + " is not supported");
  }
  if (request.model !== undefined && request.model !== declaredModel) throw new Error("model must match the selected model");
  if (request.video_operation !== undefined && request.video_operation !== "generate") throw new Error("only video_operation generate is supported");
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
  const automaticSeconds = seconds === -1 && profile.autoSeconds;
  if (!automaticSeconds && (!Number.isInteger(seconds) || seconds < profile.minSeconds || seconds > profile.maxSeconds))
    throw new Error("seconds must be an integer between " + profile.minSeconds + " and " + profile.maxSeconds);

  const aspectRatio = request.aspect_ratio === undefined ? profile.defaultAspect : request.aspect_ratio;
  if (typeof aspectRatio !== "string" || !profile.aspects.includes(aspectRatio)) throw new Error("aspect_ratio must be one of " + profile.aspects.join(", "));

  const images = roleItems(request.image_with_roles, "image_with_roles", profile.imageRoles, profile.maxImages);
  const videos = roleItems(request.video_with_roles, "video_with_roles", ["reference_video"], profile.maxVideos);
  const audios = roleItems(request.audio_with_roles, "audio_with_roles", ["reference_audio"], profile.maxAudios);
  const firstFrames = images.filter((item) => item.role === "first_frame").length;
  const lastFrames = images.filter((item) => item.role === "last_frame").length;
  const referenceImages = images.filter((item) => item.role === "reference_image").length;
  const frameMode = firstFrames > 0 || lastFrames > 0;
  if (frameMode) {
    if (firstFrames !== 1) throw new Error("image_with_roles must contain exactly one first_frame");
    if (referenceImages > 0) throw new Error("frame pair cannot be combined with reference_image");
    if (profile.framesExclusiveWithVideo && videos.length > 0) throw new Error("frame pair cannot be combined with reference_video");
  }
  if (profile.videoMode === "required" && videos.length === 0) throw new Error("with-video models require at least one reference_video");
  if (profile.videoMode === "forbidden" && videos.length > 0) throw new Error("this model does not accept reference_video");
  if (profile.audioRequiresMedia && audios.length > 0 && images.length === 0 && videos.length === 0)
    throw new Error("audio_with_roles requires image or video material");
  if (profile.adaptiveWithFrames && frameMode && aspectRatio !== "adaptive") throw new Error("frame inputs require aspect_ratio adaptive");
  if (profile.adaptiveWithoutMediaBlocked && aspectRatio === "adaptive" && images.length === 0 && videos.length === 0 && audios.length === 0)
    throw new Error("text-only generation does not support aspect_ratio adaptive");
  if (profile.maxTotalMaterials && images.length + videos.length + audios.length > profile.maxTotalMaterials)
    throw new Error("reference materials must not exceed " + profile.maxTotalMaterials + " items");

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
  if (request.audio_with_roles !== undefined) body.audio_with_roles = audios;
  return { body: body, profile: profile, videoInput: videos.length > 0 ? "video" : "none" };
}

function integerValue(value) {
  if (typeof value === "string" && value.trim()) value = Number(value);
  return Number.isInteger(value) ? value : null;
}

function normalizePaiRequest(request, declaredModel, profile) {
  for (const key of Object.keys(request)) {
    if (!REQUEST_FIELDS.includes(key)) throw new Error("field " + key + " is not supported");
  }
  if (request.model !== undefined && request.model !== declaredModel) throw new Error("model must match the selected model");
  if (request.video_operation !== undefined && request.video_operation !== "generate") throw new Error("only video_operation generate is supported");
  if (request.resolution !== undefined) throw new Error("PAI model resolution is selected by the model ID; resolution must not be sent");
  if (request.size !== undefined) throw new Error("PAI model resolution is selected by the model ID; size must not be sent");
  if (request.image_with_roles !== undefined || request.video_with_roles !== undefined || request.audio_with_roles !== undefined)
    throw new Error("PAI multimodal materials are not supported yet");
  if (typeof request.prompt !== "string" || !request.prompt.trim()) throw new Error("field prompt is required");

  const seconds = integerValue(request.seconds);
  if (request.duration !== undefined) {
    const duration = integerValue(request.duration);
    if (seconds === null || duration !== seconds) throw new Error("duration must match seconds");
  }
  if (seconds === null || seconds < profile.minSeconds || seconds > profile.maxSeconds)
    throw new Error("seconds must be an integer between " + profile.minSeconds + " and " + profile.maxSeconds);
  if (request.aspect_ratio !== undefined && (typeof request.aspect_ratio !== "string" || !request.aspect_ratio.trim()))
    throw new Error("aspect_ratio must be a non-empty string");

  const body = { model: declaredModel, prompt: request.prompt, seconds: seconds };
  if (request.aspect_ratio !== undefined) body.aspect_ratio = request.aspect_ratio;
  if (request.video_operation !== undefined) body.video_operation = "generate";
  return { body: body, profile: profile, videoInput: "none" };
}

function normalizeRequest(request, declaredModel, profileModel) {
  if (!objectValue(request)) throw new Error("request body must be an object");
  const profileName = profileModel || declaredModel;
  const profile = modelProfile(profileName);
  if (profile.billing === "pai_call" || profile.billing === "pai_second") return normalizePaiRequest(request, declaredModel, profile);
  if (profile.billing === "second") return normalizeSecondRequest(request, declaredModel, profile);
  const allowedKeys = new Set(REQUEST_FIELDS);
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

function validSeconds(value, allowZero) {
  return typeof value === "number" && Number.isFinite(value) && (allowZero ? value >= 0 : value > 0) ? value : null;
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
  const body = Object.assign({}, normalized.body, { model: upstreamModel });
  // The supplier's New API path parses the OpenAI video request DTO, where
  // seconds is a string; PAI records also use that wire shape.
  if (normalized.profile.billing === "pai_call" || normalized.profile.billing === "pai_second") {
    body.seconds = String(body.seconds);
    body.duration = normalized.body.seconds;
  }
  return {
    url: ctx.baseUrl + "/v1/videos",
    method: "POST",
    headers: { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json" },
    body: body,
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
  if (normalized.profile.billing === "pai_call") {
    return {};
  }
  if (normalized.profile.billing === "pai_second") {
    if (ctx.usagePurpose === "billing_ratios") {
      return { seconds: normalized.body.seconds };
    }
    return { output_seconds: normalized.body.seconds, input_seconds: 0 };
  }
  if (normalized.profile.billing === "second") {
    return { output_seconds: 0, input_seconds: 0 };
  }
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
  const data = unwrapData(body);
  const usage = data && data.usage;
  if (!objectValue(usage)) {
    console.log("warning: completed video response has no usable usage; actual billing facts were not reported");
    return {};
  }
  const outputSeconds = validSeconds(usage.output_seconds, false);
  if (outputSeconds !== null) {
    const facts = { output_seconds: outputSeconds };
    const inputSeconds = validSeconds(usage.input_seconds, true);
    if (inputSeconds !== null) {
      facts.input_seconds = inputSeconds;
    } else {
      console.log("warning: completed per-second video response has no valid usage.input_seconds; input video time was not reported");
    }
    return facts;
  }
  const tokens = validTotalTokens(body);
  if (tokens === null) {
    console.log("warning: completed video response has no valid usage; actual billing facts were not reported");
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
  const data = unwrapData(ctx.data) || {};
  const resultUrl = data.video_url || data.url || data.result_url;
  if (typeof resultUrl === "string" && resultUrl.trim() === resultUrl && /^https?:\/\/[^\s/?#]+(?:[/?#][^\s]*)?$/i.test(resultUrl)) {
    // PAI's CDN rejects non-browser clients, and the result URL is already
    // public/signed. Do not send the supplier API key to a foreign origin.
    return { url: resultUrl, method: ctx.clientRequest.method, headers: { "User-Agent": CONTENT_USER_AGENT } };
  }
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
      const hasMedia = normalized.body.image_with_roles || normalized.body.video_with_roles || normalized.body.audio_with_roles;
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
      // Only the measured billing facts are safe to publish.
      const usage = data.usage;
      if (objectValue(usage)) {
        const publicUsage = {};
        const total = usage.total_tokens;
        const outputSeconds = usage.output_seconds;
        const inputSeconds = usage.input_seconds;
        if (typeof total === "number" && Number.isFinite(total) && total >= 0) publicUsage.total_tokens = total;
        if (validSeconds(outputSeconds, true) !== null) publicUsage.output_seconds = outputSeconds;
        if (validSeconds(inputSeconds, true) !== null) publicUsage.input_seconds = inputSeconds;
        if (Object.keys(publicUsage).length > 0) output.usage = publicUsage;
      }
      return output;
    },
  },
};
