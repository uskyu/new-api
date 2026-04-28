export const ECOMMERCE_DETAIL_SEGMENTS = [
  {
    key: 'top',
    label: '首屏主视觉',
    description: '商品主图、核心卖点、品牌氛围和第一屏购买动机',
  },
  {
    key: 'middle',
    label: '中段卖点拆解',
    description: '功能模块、细节特写、参数表和场景说明',
  },
  {
    key: 'bottom',
    label: '尾段信任收口',
    description: '包装、保障、适用人群、礼赠或 CTA 收尾',
  },
];

export const ECOMMERCE_IMAGE_TEMPLATES = [
  {
    key: 'general-detail',
    name: '通用商品详情模板',
    tag: '全品类',
    tone: '干净、真实、转化导向',
    productType: '日用消费品',
    platform: '淘宝 / 天猫 / 京东',
    sellingPoints: '材质可靠、使用方便、细节精致、适合日常购买',
    description: '覆盖大多数普通商品，适合快速生成标准电商详情页草案。',
    modules: ['主视觉', '卖点三栏', '细节展示', '参数信息', '保障收尾'],
    stylePrompt:
      'standard Chinese ecommerce product detail page, clean white and light gray commercial layout, modular cards, realistic product photography, concise premium Chinese typography zones, strong purchase conversion structure',
  },
  {
    key: 'premium-luxury',
    name: '高端质感商品模板',
    tag: '高客单',
    tone: '高级、克制、礼赠感',
    productType: '手表 / 首饰 / 礼盒 / 高端小物',
    platform: '天猫 / 京东 / 品牌商城',
    sellingPoints: '高级材质、精密工艺、礼赠体面、质感包装',
    description: '强调材质、光影和高级感，适合手表、首饰、礼盒等商品。',
    modules: ['高级首屏', '材质工艺', '局部特写', '参数规格', '礼赠包装'],
    stylePrompt:
      'premium luxury Chinese ecommerce detail page, black silver champagne palette, refined shadows, elegant product close-ups, high-end boxed sections, luxury gift-oriented closing area',
  },
  {
    key: 'digital-tech',
    name: '3C 数码卖点模板',
    tag: '科技感',
    tone: '科技、参数、性能感',
    productType: '耳机 / 手表 / 音箱 / 数码配件',
    platform: '京东 / 天猫 / 抖音商城',
    sellingPoints: '性能强、参数清晰、续航稳定、智能体验',
    description: '突出参数、科技光效和功能拆解，适合 3C 与智能硬件。',
    modules: ['科技主视觉', '核心参数', '功能拆解', '场景体验', '对比说明'],
    stylePrompt:
      'technology ecommerce product detail page, dark blue and cyan accents, futuristic module cards, specification tables, performance callouts, precise grid layout, Chinese tech typography zones',
  },
  {
    key: 'beauty-skincare',
    name: '美妆护肤成分模板',
    tag: '功效型',
    tone: '清透、专业、成分可信',
    productType: '护肤品 / 面膜 / 个护产品',
    platform: '天猫 / 小红书 / 抖音商城',
    sellingPoints: '成分清晰、功效明确、肤感温和、使用步骤简单',
    description: '适合强调成分、功效、肤感和使用场景的美妆个护商品。',
    modules: ['功效首屏', '成分说明', '使用步骤', '肤感场景', '安心背书'],
    stylePrompt:
      'beauty skincare Chinese ecommerce detail page, soft cream and pastel palette, ingredient explanation panels, clean product close-ups, gentle professional layout, skincare benefit modules',
  },
  {
    key: 'food-trust',
    name: '食品保健信任模板',
    tag: '信任背书',
    tone: '安心、原料、品质感',
    productType: '食品 / 冲饮 / 保健品 / 零食',
    platform: '天猫 / 京东 / 抖音商城',
    sellingPoints: '原料可靠、口感自然、检测安心、适合日常囤货',
    description: '强调原料来源、口感、检测和信任背书，适合食品保健类。',
    modules: ['食欲主视觉', '原料来源', '口感卖点', '检测背书', '适用场景'],
    stylePrompt:
      'food and wellness Chinese ecommerce detail page, warm natural colors, ingredient origin panels, trust badges, appetizing product photography, clean specification and certification blocks',
  },
  {
    key: 'gift-scene',
    name: '礼品送礼场景模板',
    tag: '节日礼赠',
    tone: '温暖、体面、场景化',
    productType: '礼盒 / 香薰 / 饰品 / 节日礼品',
    platform: '淘宝 / 天猫 / 小红书',
    sellingPoints: '包装精致、送礼体面、节日氛围、情绪价值强',
    description: '强化节日、礼盒、包装和送礼场景，适合礼品类商品。',
    modules: ['送礼主视觉', '包装展示', '适用场景', '心意表达', '礼赠收尾'],
    stylePrompt:
      'gift-oriented Chinese ecommerce detail page, warm festive palette, elegant packaging display, lifestyle gifting scenarios, emotional selling point modules, polished CTA closing section',
  },
];

export const DEFAULT_ECOMMERCE_TEMPLATE_KEY = 'general-detail';
