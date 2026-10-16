# 河湾镇整体布局｜建模前设计

生成方式：OpenAI 内置图像生成，原创设计，2026-10-16。设计前已查看仓库两张参考图。首次参考图请求超时，此为纯文字提示重试的成功结果。

## 成功生成的原始提示词

Original Chinese wuxia game art concept, professional painted 3D scene design, landscape 1536x1024. Oblique orthographic view at 55 degrees from south, showing a bounded square 60m village. ONLY TWO BUILDINGS: a 14x14m two-storey white-plaster timber inn with charcoal curved tile roofs in the northwest, and a 14x14m one-storey forge with half-open work shed in the southeast. One large old jade-olive tree in northeast. Empty central spawn plaza with warm gray stone paving. Four open road entrances centered on each edge, west/east/south 8m wide and north6m wide. Low dark timber fence along perimeter with gaps for roads. Sparse small well, crates, barrels, lantern poles and stall near buildings, not on roads. Stylized handpainted Wuxia, grounded adult scale, no magic, no chibi, elegant clean shapes with detailed roof tiles and wood, ivory/umber/slate blue palette, sunlit warm amber forge. No additional houses. Soft grass edges, dusty dirt road connections, light neutral backdrop. Production design image BEFORE modeling, no UI or labels.

## 设计约束

此图是美术方向图，不是模型渲染或游戏实机截图。实际模型以任务书为准。精确位置和朝向以 `assets/source/sample-village-layout.json` 为准：客栈 Blender (-15, 15, 0)，铁匠铺 (15, -15, 0)，出生点 (0, 0, 0)。道路缺口为西、东、南 8 米，北 6 米。
