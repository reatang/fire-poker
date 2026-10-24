.PHONY: gen_icon
gen_icon:
	uv run --with pillow==12.0.0 tools/generate-brand-icon.py

.PHONY: gen_dev_icon
gen_dev_icon:
	uv run --with pillow==12.0.0 tools/generate-brand-icon.py
