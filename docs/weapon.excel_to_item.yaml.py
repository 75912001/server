#!/usr/bin/env python3
"""将8份独立武器Excel确定性生成到server/config/item.weapon.yaml."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import tempfile
import xml.etree.ElementTree as ET
import zipfile
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path


MAIN_NS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
DOC_REL_NS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
PKG_REL_NS = "http://schemas.openxmlformats.org/package/2006/relationships"
CELL_REF_RE = re.compile(r"([A-Z]+)(\d+)")


@dataclass(frozen=True)
class WeaponSpec:
    label: str
    group: str
    id_start: int
    id_end: int

    @property
    def workbook_name(self) -> str:
        return f"item.equipment.weapon.{self.label}.8.0.xlsx"

    @property
    def sheet(self) -> str:
        return f"武器.{self.label}"

    @property
    def atlas(self) -> str:
        return f"item/武器.{self.label}"


# 按协议武器类型顺序完整生成独立武器配置.
WEAPON_SPECS = (
    WeaponSpec("爪", "weaponClaw", 3900000, 3909999),
    WeaponSpec("斧", "weaponAxe", 3910000, 3919999),
    WeaponSpec("棍", "weaponStaff", 3920000, 3929999),
    WeaponSpec("枪", "weaponSpear", 3930000, 3939999),
    WeaponSpec("弓", "weaponBow", 3940000, 3949999),
    WeaponSpec("回旋镖", "weaponBoomerang", 3950000, 3959999),
    WeaponSpec("投掷斧", "weaponThrowingAxe", 3960000, 3969999),
    WeaponSpec("投掷石", "weaponThrowingStone", 3970000, 3979999),
)

WEAPON_CONFIG_HEADER = """# 武器-配置
# 本文件由 docs/weapon.excel_to_item.yaml.py 从8份独立武器Excel完整生成, 不要手工编辑.
# 仅生成同时具有武器id和原版id的行; 任一ID为空时跳过.
# 字段契约、ID区间和商店规则见 config/README.md.

"""

TEXT_FIELDS = ("name", "secretname", "effectstring")
INTEGER_FIELDS = (
    "cost",
    "level",
    "neprof",
    "otdmags",
    "otdefcs",
    "nsuit",
    "attacknum_min",
    "attacknum_max",
    "attack_min",
    "attack_max",
    "defence_min",
    "defence_max",
    "quick_min",
    "quick_max",
    "hp_min",
    "hp_max",
    "mp_min",
    "mp_max",
    "luck_min",
    "luck_max",
    "charm_min",
    "charm_max",
    "avoid_min",
    "avoid_max",
    "attrib",
    "attribvalue",
    "magicid",
    "magicusemp",
    "poison_min",
    "poison_max",
    "paralysis_min",
    "paralysis_max",
    "sleep_min",
    "sleep_max",
    "stone_min",
    "stone_max",
    "drunk_min",
    "drunk_max",
    "confusion_min",
    "confusion_max",
    "critical_min",
    "critical_max",
)
UNSIGNED_FIELDS = {
    "cost",
    "level",
    "neprof",
    "nsuit",
    "attacknum_min",
    "attacknum_max",
    "attrib",
    "attribvalue",
    "magicid",
    "magicusemp",
}
RANGE_PREFIXES = (
    "attacknum",
    "attack",
    "defence",
    "quick",
    "hp",
    "mp",
    "luck",
    "charm",
    "avoid",
    "poison",
    "paralysis",
    "sleep",
    "stone",
    "drunk",
    "confusion",
    "critical",
)
REQUIRED_HEADERS = {"帧ID", "武器id", "id", "name"}
IGNORED_HEADERS = {"帧图片", "weaponLevel", "列1"}
GUARDED_UNSUPPORTED_HEADERS = {"hirt", "neguard"}
ALLOWED_HEADERS = (
    REQUIRED_HEADERS
    | IGNORED_HEADERS
    | GUARDED_UNSUPPORTED_HEADERS
    | set(TEXT_FIELDS)
    | set(INTEGER_FIELDS)
)


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def is_blank(value: object | None) -> bool:
    return value is None or str(value).strip() == ""


def column_number(cell_ref: str) -> int:
    match = CELL_REF_RE.fullmatch(cell_ref)
    if match is None:
        raise ValueError(f"无效单元格引用: {cell_ref}")
    result = 0
    for char in match.group(1):
        result = result * 26 + ord(char) - ord("A") + 1
    return result


def normalize_number(raw_value: str) -> int | float:
    try:
        return int(raw_value)
    except ValueError:
        return float(raw_value)


def read_shared_strings(archive: zipfile.ZipFile) -> list[str]:
    if "xl/sharedStrings.xml" not in archive.namelist():
        return []
    root = ET.fromstring(archive.read("xl/sharedStrings.xml"))
    return [
        "".join(node.text or "" for node in item.iter(f"{{{MAIN_NS}}}t"))
        for item in root.findall(f"{{{MAIN_NS}}}si")
    ]


def resolve_sheet_path(archive: zipfile.ZipFile, sheet_name: str) -> str:
    workbook_root = ET.fromstring(archive.read("xl/workbook.xml"))
    sheets = workbook_root.find(f"{{{MAIN_NS}}}sheets")
    if sheets is None:
        raise ValueError("Excel工作簿缺少sheets节点")

    relationship_id = None
    for sheet in sheets:
        if sheet.attrib.get("name") == sheet_name:
            relationship_id = sheet.attrib[f"{{{DOC_REL_NS}}}id"]
            break
    if relationship_id is None:
        raise ValueError(f"源文档缺少工作表: {sheet_name}")

    relationships = ET.fromstring(archive.read("xl/_rels/workbook.xml.rels"))
    for relationship in relationships.findall(f"{{{PKG_REL_NS}}}Relationship"):
        if relationship.attrib.get("Id") == relationship_id:
            target = relationship.attrib["Target"].replace("\\", "/").lstrip("/")
            return target if target.startswith("xl/") else f"xl/{target}"
    raise ValueError(f"工作表关系不存在: {sheet_name}")


def read_sheet_rows(
    archive: zipfile.ZipFile, sheet_name: str
) -> list[tuple[int, list[object | None]]]:
    shared_strings = read_shared_strings(archive)
    root = ET.fromstring(archive.read(resolve_sheet_path(archive, sheet_name)))
    result: list[tuple[int, list[object | None]]] = []
    for row in root.findall(f".//{{{MAIN_NS}}}sheetData/{{{MAIN_NS}}}row"):
        values: dict[int, object] = {}
        for cell in row.findall(f"{{{MAIN_NS}}}c"):
            value_node = cell.find(f"{{{MAIN_NS}}}v")
            inline_node = cell.find(f"{{{MAIN_NS}}}is")
            if value_node is None and inline_node is None:
                continue
            cell_type = cell.attrib.get("t")
            if cell_type == "inlineStr":
                value = "" if inline_node is None else "".join(
                    node.text or "" for node in inline_node.iter(f"{{{MAIN_NS}}}t")
                )
            else:
                raw_value = "" if value_node is None else value_node.text or ""
                if cell_type == "s":
                    value = shared_strings[int(raw_value)]
                elif cell_type == "b":
                    value = raw_value == "1"
                elif cell_type in {"str", "e"}:
                    value = raw_value
                else:
                    value = normalize_number(raw_value)
            values[column_number(cell.attrib["r"])] = value
        if values:
            width = max(values)
            row_number = int(row.attrib.get("r", len(result) + 1))
            result.append((row_number, [values.get(index) for index in range(1, width + 1)]))
    return result


def parse_integer(
    value: object | None,
    *,
    row_number: int,
    field: str,
    required: bool = False,
) -> int:
    if is_blank(value):
        if required:
            raise ValueError(f"Excel第{row_number}行缺少{field}")
        return 0
    if isinstance(value, bool):
        raise ValueError(f"Excel第{row_number}行{field}不是整数: {value!r}")
    try:
        number = Decimal(str(value).strip())
    except (InvalidOperation, ValueError) as error:
        raise ValueError(f"Excel第{row_number}行{field}不是整数: {value!r}") from error
    integral = number.to_integral_value()
    if number != integral:
        raise ValueError(f"Excel第{row_number}行{field}不是整数: {value!r}")
    return int(integral)


def quote_yaml_string(value: str) -> str:
    return json.dumps(value, ensure_ascii=False)


def load_source_records(workbook: Path, spec: WeaponSpec) -> tuple[list[dict], list[str]]:
    before = workbook.stat()
    with zipfile.ZipFile(workbook) as archive:
        rows = read_sheet_rows(archive, spec.sheet)
    after = workbook.stat()
    if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
        raise RuntimeError(f"读取期间Excel文件发生变化: {workbook}")
    if not rows or rows[0][0] != 1:
        raise ValueError(f"工作表第1行必须是字段标题: {spec.sheet}")

    headers = [str(value).strip() if value is not None else "" for value in rows[0][1]]
    duplicate_headers = sorted({header for header in headers if header and headers.count(header) > 1})
    if duplicate_headers:
        raise ValueError(f"工作表存在重复字段: {duplicate_headers}")
    missing_headers = sorted(REQUIRED_HEADERS - set(headers))
    if missing_headers:
        raise ValueError(f"工作表缺少必填字段: {missing_headers}")
    unknown_headers = sorted({header for header in headers if header} - ALLOWED_HEADERS)
    if unknown_headers:
        raise ValueError(f"工作表包含未映射字段: {unknown_headers}")

    records = []
    for row_number, row in rows:
        if row_number < 3:
            continue
        padded = row + [None] * (len(headers) - len(row))
        record = dict(zip(headers, padded))
        record["_excel_row"] = row_number
        records.append(record)
    return records, headers


def validate_record_ranges(values: dict[str, int], *, row_number: int) -> None:
    for prefix in RANGE_PREFIXES:
        minimum = values.get(f"{prefix}_min", 0)
        maximum = values.get(f"{prefix}_max", 0)
        if minimum > maximum:
            raise ValueError(
                f"Excel第{row_number}行{prefix}范围无效: min={minimum}, max={maximum}"
            )


def render_group(
    records: list[dict],
    headers: list[str],
    *,
    workbook: Path,
    spec: WeaponSpec,
    newline: str,
) -> tuple[str, dict]:
    lines = [
        f"  # {spec.sheet}",
        f"  # 数据源: docs/{workbook.name}, 工作表: {spec.sheet}",
        f"  {spec.group}:",
    ]
    seen_weapon_ids: set[int] = set()
    seen_original_ids: set[int] = set()
    skipped: list[dict] = []

    for record in records:
        row_number = int(record["_excel_row"])
        if is_blank(record.get("武器id")):
            skipped.append(
                {
                    "excel_row": row_number,
                    "original_id": record.get("id"),
                    "name": record.get("name"),
                    "reason": "武器id为空",
                }
            )
            continue
        if is_blank(record.get("id")):
            skipped.append(
                {
                    "excel_row": row_number,
                    "weapon_id": record.get("武器id"),
                    "name": record.get("name"),
                    "reason": "id为空",
                }
            )
            continue

        weapon_id = parse_integer(
            record.get("武器id"), row_number=row_number, field="武器id", required=True
        )
        original_id = parse_integer(
            record.get("id"), row_number=row_number, field="id", required=True
        )
        sprite = parse_integer(
            record.get("帧ID"), row_number=row_number, field="帧ID", required=True
        )
        name = str(record.get("name") or "").strip()

        if not spec.id_start <= weapon_id <= spec.id_end:
            raise ValueError(
                f"Excel第{row_number}行武器id越界: {weapon_id}, "
                f"允许范围[{spec.id_start},{spec.id_end}]"
            )
        if weapon_id in seen_weapon_ids:
            raise ValueError(f"Excel第{row_number}行武器id重复: {weapon_id}")
        if original_id in seen_original_ids:
            raise ValueError(f"Excel第{row_number}行原版id重复: {original_id}")
        if sprite <= 0:
            raise ValueError(f"Excel第{row_number}行帧ID必须大于0: {sprite}")
        if not name:
            raise ValueError(f"Excel第{row_number}行名称为空: 武器id={weapon_id}")

        unsupported_nonzero = []
        for field in GUARDED_UNSUPPORTED_HEADERS & set(headers):
            value = parse_integer(record.get(field), row_number=row_number, field=field)
            if value != 0:
                unsupported_nonzero.append(f"{field}={value}")
        if unsupported_nonzero:
            raise ValueError(
                f"Excel第{row_number}行包含服务端尚未支持的非零字段: "
                + ", ".join(sorted(unsupported_nonzero))
            )

        integer_values: dict[str, int] = {}
        for field in INTEGER_FIELDS:
            value = parse_integer(record.get(field), row_number=row_number, field=field)
            if field in UNSIGNED_FIELDS and value < 0:
                raise ValueError(f"Excel第{row_number}行{field}不能为负数: {value}")
            integer_values[field] = value

        if integer_values["neprof"] not in {0, 1, 2, 3}:
            raise ValueError(f"Excel第{row_number}行neprof无效: {integer_values['neprof']}")
        if integer_values["attrib"] not in {0, 1, 2, 3, 4}:
            raise ValueError(f"Excel第{row_number}行attrib无效: {integer_values['attrib']}")
        raw_attribute_value = integer_values["attribvalue"]
        if raw_attribute_value % 10 != 0:
            raise ValueError(
                f"Excel第{row_number}行attribvalue不能按原版比例除以10: "
                f"{raw_attribute_value}"
            )
        integer_values["attribvalue"] = raw_attribute_value // 10
        if integer_values["attribvalue"] > 10:
            raise ValueError(
                f"Excel第{row_number}行attribvalue转换后超过10: "
                f"{integer_values['attribvalue']}"
            )
        if integer_values["attrib"] == 0 and integer_values["attribvalue"] != 0:
            raise ValueError(
                f"Excel第{row_number}行无元素但attribvalue非零: "
                f"{integer_values['attribvalue']}"
            )
        validate_record_ranges(integer_values, row_number=row_number)

        seen_weapon_ids.add(weapon_id)
        seen_original_ids.add(original_id)
        lines.append(f"    {weapon_id}:")
        lines.append(f"      name: {quote_yaml_string(name)}")
        for field in TEXT_FIELDS[1:]:
            value = str(record.get(field) or "")
            if value:
                lines.append(f"      {field}: {quote_yaml_string(value)}")
        lines.append(f"      atlas: {quote_yaml_string(spec.atlas)}")
        lines.append(f"      sprite: {sprite}")
        for field in INTEGER_FIELDS:
            value = integer_values[field]
            if value != 0:
                lines.append(f"      {field}: {value}")

    if not seen_weapon_ids:
        raise ValueError(f"源文档没有可生成的{spec.sheet}记录")

    summary = {
        "source_document": str(workbook),
        "source_sheet": spec.sheet,
        "target_group": spec.group,
        "source_records": len(records),
        "generated_records": len(seen_weapon_ids),
        "skipped_records": skipped,
        "id_min": min(seen_weapon_ids),
        "id_max": max(seen_weapon_ids),
        "source_sha256": sha256_bytes(workbook.read_bytes()),
    }
    return newline.join(lines) + newline, summary



def write_bytes_atomic(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="wb", dir=path.parent, prefix=f".{path.name}.", suffix=".tmp", delete=False
    ) as output:
        temporary_path = Path(output.name)
        output.write(data)
        output.flush()
        os.fsync(output.fileno())
    try:
        os.replace(temporary_path, path)
    except BaseException:
        temporary_path.unlink(missing_ok=True)
        raise


def save_backup(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        if path.read_bytes() != data:
            raise RuntimeError(f"备份文件已存在但内容不同: {path}")
        return
    with path.open("xb") as output:
        output.write(data)


def parse_args() -> argparse.Namespace:
    server_root = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(
        description="将docs目录中的8份独立武器Excel生成到config/item.weapon.yaml."
    )
    parser.add_argument(
        "--config",
        type=Path,
        default=server_root / "config" / "item.weapon.yaml",
        help="目标item.weapon.yaml, 默认使用server/config/item.weapon.yaml.",
    )
    parser.add_argument(
        "--candidate",
        type=Path,
        help="可选的候选输出路径, 写入正式配置前用于人工比较.",
    )
    parser.add_argument(
        "--backup-dir",
        type=Path,
        default=server_root.parent / ".codex-tmp" / "item_yaml_backups",
        help="正式写入前的备份目录.",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="只校验并生成摘要, 不写入正式配置.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    docs_dir = Path(__file__).resolve().parent
    config = args.config.resolve()
    config_existed = config.is_file()
    config_before = config.read_bytes() if config_existed else b""
    newline = "\r\n" if b"\r\n" in config_before else "\n"
    blocks = []
    summaries = []

    # 所有Excel先完成读取、字段和范围校验; 任一失败时目标配置保持不变.
    for spec in WEAPON_SPECS:
        workbook = docs_dir / spec.workbook_name
        if not workbook.is_file():
            raise FileNotFoundError(f"源文档不存在: {workbook}")
        records, headers = load_source_records(workbook, spec)
        block, summary = render_group(
            records,
            headers,
            workbook=workbook,
            spec=spec,
            newline=newline,
        )
        blocks.append(block)
        summaries.append(summary)

    candidate_bytes = (WEAPON_CONFIG_HEADER + "items:" + newline + "".join(blocks)).encode("utf-8")
    changed = candidate_bytes != config_before
    if args.candidate is not None:
        write_bytes_atomic(args.candidate.resolve(), candidate_bytes)

    backup_path = None
    if not args.check and changed:
        # 避免运行期间外部编辑被最终原子替换覆盖.
        if config_existed and config.read_bytes() != config_before:
            raise RuntimeError(f"生成期间目标配置发生变化: {config}")
        if not config_existed and config.exists():
            raise RuntimeError(f"生成期间目标配置被外部创建: {config}")
        if config_existed:
            backup_path = args.backup_dir.resolve() / (
                f"{config.name}.before.weaponExcelBatch.{sha256_bytes(config_before)[:12]}.yaml"
            )
            save_backup(backup_path, config_before)
        write_bytes_atomic(config, candidate_bytes)
        if config.read_bytes() != candidate_bytes:
            raise RuntimeError(f"写入后内容校验失败: {config}")

    result = {
        "target_config": str(config),
        "mode": "check" if args.check else "write",
        "config_changed": changed,
        "config_before_sha256": sha256_bytes(config_before) if config_existed else None,
        "config_after_sha256": sha256_bytes(candidate_bytes),
        "backup": None if backup_path is None else str(backup_path),
        "groups": summaries,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
