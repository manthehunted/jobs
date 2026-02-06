import argparse
from docx import Document
from pathlib import Path
import logging
import json
import subprocess
import shlex
import sys
import re

logging.basicConfig(stream=sys.stdout, level=logging.INFO,
                    format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

# The regex pattern covers most common ANSI escape sequences
ANSI_ESCAPE = re.compile(r'(\x9B|\x1B\[)[0-?]*[ -/]*[@-~]')

MAKE_RESUME = "follow @tmpl/Prompt_Resume.md and make a json extension with a file name containing company name using @tmpl/ResumeContents.md and {job_description}"
MAKE_COVER = "make a cover letter with txt extension and file name containing company name, {company}, using {resume} and {job_description}"


def strip_ansi_codes(s):
    return ANSI_ESCAPE.sub('', s)


def insert(name):
    fs = Path(name)
    assert fs.exists(), fs

    name = name.replace(".json", "")
    newname = f"{name}.docx"

    with open(fs) as f:
        changes = json.load(f)
        assert changes["summary"]
        assert changes["experiences"]

    doc = Document("./tmpl/ResumeTmpl.docx")

    paras = doc.paragraphs
    ch = changes["experiences"]

    for para in paras:
        if "INSERT" not in para.text:
            continue

        if "summary" in para.text:
            by = changes["summary"]
            para.text = by
        elif "experience" in para.text:
            para.text = ch.pop(0)
            assert name not in para.text, f"{name} exists"

    doc.save(newname)
    logger.info("made %s" % newname)
    return newname


def get_company(s: str) -> str:
    company = Path(s)
    company = company.name.replace(company.suffix, "")
    return company


def make_cover(fs_pdf, fs_job, company):
    prompt = MAKE_COVER.format(
        company=company, resume=fs_pdf, job_description=fs_job)
    cmd = 'opencode run "{prompt}"'.format(prompt=prompt)
    cmd = shlex.split(cmd)

    logger.info("write coverletter")
    output = subprocess.run(cmd, capture_output=True)

    name = check_output(output, "txt")
    logger.info("made %s" % name)
    return name


def check_output(output, ext):
    captured = list(filter(lambda x: b"Write" in x,
                           output.stderr.split(b"\n")))
    assert captured, "making resume for " + company
    captured = captured[0]
    assert ("{}".format(ext)).encode(
        "utf-8") in captured, f"making {ext} for " + company
    strs = list(filter(lambda x: f"{ext}" in x, strip_ansi_codes(
        captured.decode("utf-8")).split("Write")))
    current = Path(strs[0].strip())
    assert current.exists(), current
    return str(current)


def text_to_json(fs_job_description: str):
    prompt = MAKE_RESUME.format(job_description=fs_job_description)
    cmd = 'opencode run "{prompt}"'.format(prompt=prompt)
    cmd = shlex.split(cmd)
    logger.info("cooking content")
    output = subprocess.run(cmd, capture_output=True)

    name = check_output(output, "json")
    logger.info("made %s" % name)
    return name


def json_to_docx(fs_json: str):
    name = insert(fs_json)
    logger.info("made %s" % name)
    return name


def docx_to_pdf(fs_docx: str):
    path = fs_docx
    cmd = shlex.split(
        f"/Applications/LibreOffice.app/Contents/MacOS/soffice --headless --convert-to pdf {path}")
    subprocess.run(cmd,)
    logger.info("made %s" % path.replace(".docx", ".pdf"))


# txt -> json -> docx
if __name__ == "__main__":
    parser = argparse.ArgumentParser()

    parser.add_argument("name", )

    args = parser.parse_args()
    name = args.name
    company = get_company(name)

    if name.endswith("txt"):
        name = text_to_json(name)

    if name.endswith("json"):
        name = json_to_docx(name)

    if name.endswith("docx"):
        name = docx_to_pdf(name)

    make_cover(name, f"job_descriptions/{company}.txt", company)

    names = list(Path(".").glob("*{name}*.*".format(name=company)))
    if names:
        names = " ".join(map(str, names))
        cmd = shlex.split(f"mv {names} ./resumes")
        subprocess.run(cmd,)
