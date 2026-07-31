# 石器时代 8.0 道具资源映射参考

## 结果概览

本参考资料将道具 BMP 文件名解析为资源帧号, 经 ADRN 映射为 `itemset6.txt` 的
`imagenumber`, 再关联内部道具 ID 和完整配置字段.

- 扫描 BMP 资源: 2782 个.
- 成功关联道具记录的资源帧: 1957 个.
- 当前 `itemset6.txt` 无对应记录的资源帧: 825 个.
- 展开一对多关系后的道具详情: 15054 条.
- `itemset6.txt` 有效记录: 15164 条, 每条均为 94 个原始字段.

输出文件:

- `item_frame_mapping_8_0.csv`: 每个资源帧一行的映射关系表.
- `item_detail_8_0.csv`: 每个“资源帧 + 道具”组合一行的完整详情表.
- `item_reference_8_0.xlsx`: 带筛选、冻结表头和格式的 Excel 工作簿.
- `generate_item_reference_8_0.py`: 可重复生成两份 CSV 的离线脚本.

CSV 使用 UTF-8 with BOM 编码, 可由 Excel 直接打开且不会出现中文乱码.

## 数据来源

- 道具图片根目录: `D:\石器资源\.资源\道具`.
- ADRN: `D:\csa_8.0\data\adrn_136.bin`.
- 道具配置: `D:\csa_8.0\gmsv\data\itemset6.txt`.
- 字段定义参考:
  `D:\石器资源\石器时代8.5的服务端源码LINUX下开发\85\gmsv\item\item.c`.

`itemset6.txt` 的正确文本编码为 GB18030. 生成器以 GB18030 严格解码, 输出再统一为
UTF-8.

## 映射关系

当前客户端数据实测采用以下链路:

```text
BMP 数字文件名
  -> 资源帧号
  -> adrn_136.bin 第 frame_id 条 80 字节记录
  -> 记录内偏移 76-79 的 uint32 little-endian
  -> itemset6.txt 的 imagenumber
  -> itemset6.txt 的内部道具 ID 和详细字段
```

等价偏移公式:

```text
imagenumber_offset = frame_id * 80 + 76
```

该结论已用多个已知道具交叉验证. 这是针对当前 `adrn_136.bin` 的实测格式, 更换客户端
版本或 ADRN 文件后必须重新用已知道具验证, 不能直接假定记录布局不变.

## 已知映射验证

| 资源帧号 | ADRN 图号 | 目标内部 ID | 目标道具 | 同图号记录数 |
| ---: | ---: | ---: | --- | ---: |
| 6505 | 22038 | 18537 | 萨姆吉尔的首饰 | 8 |
| 6548 | 23012 | 2400 | 阿布的水 | 3 |
| 233136 | 24345 | 19646 | 灵力铠 | 4 |

源配置使用“铠”字, 因此资料中保留 `灵力铠`, 不改写为用户口述的“灵力凯”.

### 6505, 萨姆吉尔的首饰

目标记录:

- 内部道具 ID: 18537.
- `imagenumber`: 22038.
- 名称: 萨姆吉尔的首饰.
- 效果文本: `萨姆吉尔的首饰 魅 +5 复活光精灵 Lv1`.
- `type`: 10.
- `charm_min/charm_max`: 5/5.
- `magicid/magicprob/magicusemp`: 280/100/20.

同图号全部记录:

| 内部 ID | 名称 |
| ---: | --- |
| 1361 | 红色美丽首饰 |
| 11051 | 合成首饰 A3 |
| 11056 | 合成首饰 [地] A3 |
| 11061 | 合成首饰 [水] A3 |
| 11066 | 合成首饰 [火] A3 |
| 11071 | 合成首饰 [风] A3 |
| 18537 | 萨姆吉尔的首饰 |
| 20530 | 亚伊欧A3首饰2 |

### 6548, 阿布的水

目标记录:

- 内部道具 ID: 2400.
- `imagenumber`: 23012.
- 名称: 阿布的水.
- 效果文本: `气力100前後回复`.
- 参数: `气100`.
- 使用函数: `ITEM_useRecovery`.
- `cost/type/target`: 100/16/1.

同图号全部记录:

| 内部 ID | 名称 |
| ---: | --- |
| 2400 | 阿布的水 |
| 2768 | 群青的水 |
| 19563 | 冰轮的调和水 |

### 233136, 灵力铠

目标记录:

- 内部道具 ID: 19646.
- `imagenumber`: 24345.
- 名称: 灵力铠.
- 效果文本: `带有地水火风灵力的护铠 防+48 敏-12`.
- `type`: 7.
- `defence_min/defence_max`: 48/48.
- `quick_min/quick_max`: -12/-12.

同图号全部记录:

| 内部 ID | 名称 |
| ---: | --- |
| 1292 | 真．灵力铠 |
| 1623 | 新手真．灵力铠 |
| 19646 | 灵力铠 |
| 29150 | 豪豪的凱 |

## CSV 字段

### 映射表

- 资源路径: `资源相对路径`, `资源分类`, `资源文件名`.
- 映射键: `资源帧号`, `ADRN图号`.
- 结果: `匹配状态`, `匹配道具数`.
- 一对多摘要: `内部道具ID列表`, `道具名称列表`, `itemset源行列表`.

### 详情表

详情表首先保留资源帧、ADRN 图号、同图号匹配数和 `itemset` 源行号, 随后展开
`itemset6.txt` 的全部 94 个原始字段.

服务器源码中共有 79 个逻辑配置项. 其中 15 个 `ITEM_getRandomValue` 项在文本内各占
最小值和最大值两列, 因此原始文本实际为 94 列. 详情表将这些列明确展开为
`attack_min/attack_max`, `defence_min/defence_max` 等字段, 避免后续字段错位.

字段主要分组:

- 基本资料: `name`, `secretname`, `effectstring`, `argument`, `id`, `imagenumber`,
  `cost`, `type`, `fieldtype`, `target`, `level`.
- 回调函数: `initfunc` 至 `relifefunc`, 以及使用、装备、丢弃和拾取相关函数.
- 装备属性: 攻击次数、攻击、防御、敏捷、HP、MP、运气、魅力、回避、属性值.
- 魔法和异常状态: `magicid`, `magicprob`, `magicusemp`, 毒、麻痹、睡眠、石化、
  酒醉、混乱、必杀等最小值和最大值.
- 行为限制: 登出掉落、落地消失、叠加、宠物邮件、合并方向等标志.
- 合成材料: `ingname0/ingvalue0` 至 `ingname4/ingvalue4`.

## 边界和注意事项

- `imagenumber` 不是唯一道具键. 多个道具可以共享同一图片, 输出必须保留全部匹配.
- “无道具记录”只表示当前 `itemset6.txt` 未使用该 ADRN 图号. 资源可能属于其他版本、
  其他配置文件或当前未启用内容, 不能直接判定为无效图片.
- 本次未使用旧的 `history.txt` 作为主映射依据, 因为当前 ADRN 与当前
  `itemset6.txt` 的直接关联更可靠.
- 详情表保留 `itemset源行`, 可回到原始配置逐条核对.

## 重新生成

在 `server` 目录执行:

```bash
python docs/generate_item_reference_8_0.py \
  --resource-dir 'D:/石器资源/.资源/道具' \
  --adrn 'D:/csa_8.0/data/adrn_136.bin' \
  --itemset 'D:/csa_8.0/gmsv/data/itemset6.txt' \
  --output-dir docs
```

脚本只读取资源、ADRN 和 `itemset6.txt`, 只写入指定输出目录.
