package browser

const (
	fixedHelperSetInputValue = `function setValue(value) {
	const el = this;
	const tagName = el.tagName.toLowerCase();
	if (tagName === 'input' || tagName === 'textarea') {
		const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
			window.HTMLInputElement.prototype, 'value'
		) || Object.getOwnPropertyDescriptor(
			window.HTMLTextAreaElement.prototype, 'value'
		);
		if (nativeInputValueSetter && nativeInputValueSetter.set) {
			nativeInputValueSetter.set.call(el, value);
		} else {
			el.value = value;
		}
		el.dispatchEvent(new Event('input', { bubbles: true }));
		el.dispatchEvent(new Event('change', { bubbles: true }));
	} else if (el.isContentEditable) {
		el.textContent = value;
		el.dispatchEvent(new Event('input', { bubbles: true }));
		el.dispatchEvent(new Event('change', { bubbles: true }));
	}
}`

	fixedHelperSetSelectValue = `function setSelectValue(value) {
	const el = this;
	el.value = value;
	el.dispatchEvent(new Event('input', { bubbles: true }));
	el.dispatchEvent(new Event('change', { bubbles: true }));
}`

	fixedHelperTriggerClick = `function triggerClick() {
	this.click();
}`

	fixedHelperGetInputSummary = `function getInputSummary() {
	const el = this;
	const type = (el.type || '').toLowerCase();
	if (type === 'password') {
		return '[value=<redacted>]';
	}
	const val = el.value || '';
	return val.substring(0, 256);
}`

	fixedHelperIsVisible = `function isVisible() {
	const el = this;
	if (el.offsetParent === null && el.tagName.toLowerCase() !== 'body') {
		const style = window.getComputedStyle(el);
		if (style.display === 'none' || style.visibility === 'hidden') {
			return false;
		}
	}
	return true;
}`

	fixedHelperIsInteractable = `function isInteractable() {
	const el = this;
	if (el.disabled || el.readOnly) {
		return false;
	}
	const style = window.getComputedStyle(el);
	if (style.pointerEvents === 'none') {
		return false;
	}
	return true;
}`
)

type InteractionHelpers struct{}

func NewInteractionHelpers() *InteractionHelpers {
	return &InteractionHelpers{}
}

func (h *InteractionHelpers) SetInputValue() string {
	return fixedHelperSetInputValue
}

func (h *InteractionHelpers) SetSelectValue() string {
	return fixedHelperSetSelectValue
}

func (h *InteractionHelpers) TriggerClick() string {
	return fixedHelperTriggerClick
}

func (h *InteractionHelpers) GetInputSummary() string {
	return fixedHelperGetInputSummary
}

func (h *InteractionHelpers) IsVisible() string {
	return fixedHelperIsVisible
}

func (h *InteractionHelpers) IsInteractable() string {
	return fixedHelperIsInteractable
}
