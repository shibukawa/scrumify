// The one script of the application. Pages are plain server-rendered markup;
// this file turns data attributes on that markup into calls to the JSON API.
//
//   data-autosave="/api/…" data-field="title"   save a text field as it is typed
//   data-post="/api/…" data-name="status"       post a select's value on change
//   data-post="/api/…" data-param-x="…"         post fixed values on click
//   form data-api="/api/…" data-ints="a,b"      post a form as JSON
//   data-drag-id="12" / data-drop="/api/…/:id/…" data-param-x="…"
//
// A change that alters what the page shows ends with refresh(), which asks the
// server for the page again and lets the runtime swap what differs.

const SAVE_DELAY_MS = 600;

function csrf() {
	const meta = document.querySelector('meta[name="pw-runtime"]');
	if (!meta) return {};
	try {
		const config = JSON.parse(meta.content);
		return { [config.csrfHeader]: config.csrf };
	} catch {
		return {};
	}
}

async function post(url, body) {
	const response = await fetch(url, {
		method: "POST",
		headers: { "Content-Type": "application/json", Accept: "application/json", ...csrf() },
		body: JSON.stringify(body ?? {}),
	});
	if (!response.ok) {
		let detail = response.statusText;
		try {
			const problem = await response.json();
			detail = problem.detail || problem.message || problem.title || detail;
		} catch {
			// The status text is all there is.
		}
		showError(detail);
		throw new Error(detail);
	}
	return response.status === 204 ? null : response.json();
}

function showError(message) {
	let box = document.getElementById("scrumify-error");
	if (!box) {
		box = document.createElement("div");
		box.id = "scrumify-error";
		box.setAttribute("role", "alert");
		box.style.cssText =
			"position:fixed;right:1rem;bottom:1rem;z-index:50;max-width:24rem;padding:.75rem 1rem;" +
			"border-radius:.5rem;background:#b91c1c;color:#fff;font-size:.875rem;box-shadow:0 4px 12px rgba(0,0,0,.2)";
		document.body.append(box);
	}
	box.textContent = message;
	clearTimeout(showError.timer);
	showError.timer = setTimeout(() => box.remove(), 6000);
}

// params reads data-param-* attributes into a request body: data-param-before-id="3"
// becomes { beforeId: 3 }. A value made only of digits is sent as a number.
function params(element) {
	const body = {};
	for (const [key, raw] of Object.entries(element.dataset)) {
		if (!key.startsWith("param") || key.length === 5) continue;
		const name = key[5].toLowerCase() + key.slice(6);
		body[name] = /^\d+$/.test(raw) ? Number(raw) : raw;
	}
	return body;
}

// --- autosave ---------------------------------------------------------------

const pending = new Map(); // element -> timer
const inFlight = new Set();

function saveNow(element) {
	clearTimeout(pending.get(element));
	pending.delete(element);
	const value = element.value;
	if (element.dataset.saved === value) return Promise.resolve();
	element.dataset.saveState = "saving";
	const request = post(element.dataset.autosave, { field: element.dataset.field, value })
		.then(() => {
			element.dataset.saved = value;
			element.dataset.saveState = element.value === value ? "saved" : "dirty";
		})
		.catch(() => {
			element.dataset.saveState = "error";
		})
		.finally(() => inFlight.delete(request));
	inFlight.add(request);
	return request;
}

async function flush() {
	await Promise.all([...pending.keys()].map(saveNow));
	await Promise.all([...inFlight]);
}

document.addEventListener("input", (event) => {
	const element = event.target.closest?.("[data-autosave]");
	if (!element) return;
	if (element.dataset.saved === undefined) element.dataset.saved = element.defaultValue;
	element.dataset.saveState = "dirty";
	if (element.dataset.mirror) {
		for (const mirror of document.querySelectorAll(`[data-mirror-of="${element.dataset.mirror}"]`)) {
			mirror.textContent = element.value;
		}
	}
	clearTimeout(pending.get(element));
	pending.set(element, setTimeout(() => saveNow(element), SAVE_DELAY_MS));
});

document.addEventListener("focusout", (event) => {
	const element = event.target.closest?.("[data-autosave]");
	if (element && pending.has(element)) saveNow(element);
});

document.addEventListener("keydown", (event) => {
	// Enter in a single-line field means "done", not "submit".
	if (event.key === "Enter" && event.target.matches?.("input[data-autosave]")) {
		event.preventDefault();
		event.target.blur();
	}
});

window.addEventListener("pagehide", () => {
	for (const element of pending.keys()) {
		const body = JSON.stringify({ field: element.dataset.field, value: element.value });
		fetch(element.dataset.autosave, {
			method: "POST",
			keepalive: true,
			headers: { "Content-Type": "application/json", ...csrf() },
			body,
		});
	}
});

// --- refresh ----------------------------------------------------------------

async function refresh() {
	await flush();
	const here = location.pathname + location.search;
	const runtime = window.popcornweb;
	if (runtime?.navigate) {
		try {
			await runtime.navigate(here);
			return;
		} catch {
			// Fall through to a full load.
		}
	}
	location.reload();
}

// --- selects and buttons ----------------------------------------------------

document.addEventListener("change", async (event) => {
	const select = event.target.closest?.("select[data-post]");
	if (!select) return;
	const raw = select.value;
	const value = select.dataset.int !== undefined ? Number(raw) : raw;
	try {
		await post(select.dataset.post, { ...params(select), [select.dataset.name]: value });
	} finally {
		await refresh();
	}
});

document.addEventListener("click", async (event) => {
	const button = event.target.closest?.("button[data-post]");
	if (!button) return;
	event.preventDefault();
	if (button.dataset.confirm && !confirm(button.dataset.confirm)) return;
	button.disabled = true;
	try {
		await flush();
		await post(button.dataset.post, params(button));
		if (button.dataset.then) {
			location.assign(button.dataset.then);
			return;
		}
	} catch {
		button.disabled = false;
		return;
	}
	await refresh();
});

// --- forms ------------------------------------------------------------------

document.addEventListener("submit", async (event) => {
	const form = event.target.closest?.("form[data-api]");
	if (!form) return;
	// These forms have no URL of their own to submit to. Stopping the event
	// here, in the capture phase, keeps the framework runtime from treating
	// one as a GET form and putting its fields in the address bar.
	event.preventDefault();
	event.stopPropagation();
	const ints = new Set((form.dataset.ints || "").split(",").filter(Boolean));
	const body = {};
	for (const [name, value] of new FormData(form)) {
		body[name] = ints.has(name) ? Number(value) : value;
	}
	const focusName = form.dataset.refocus;
	try {
		const result = await post(form.dataset.api, body);
		form.reset();
		if (form.dataset.then && result?.id) {
			location.assign(form.dataset.then.replace(":id", result.id));
			return;
		}
	} catch {
		return;
	}
	await refresh();
	if (focusName) document.querySelector(`[data-refocus-target="${focusName}"]`)?.focus();
}, true);

// --- drag and drop ----------------------------------------------------------

let dragged = null;

document.addEventListener("dragstart", (event) => {
	const source = event.target.closest?.("[data-drag-id]");
	if (!source) return;
	dragged = { id: source.dataset.dragId, kind: source.dataset.dragKind || "" };
	event.dataTransfer.effectAllowed = "move";
	event.dataTransfer.setData("text/plain", dragged.id);
	source.dataset.dragging = "true";
});

document.addEventListener("dragend", (event) => {
	const source = event.target.closest?.("[data-drag-id]");
	if (source) delete source.dataset.dragging;
	for (const zone of document.querySelectorAll("[data-drop-over]")) delete zone.dataset.dropOver;
	dragged = null;
});

function dropZone(event) {
	const zone = event.target.closest?.("[data-drop]");
	if (!zone || !dragged) return null;
	if (zone.dataset.dropKind && zone.dataset.dropKind !== dragged.kind) return null;
	if (zone.dataset.dragId === dragged.id) return null;
	return zone;
}

document.addEventListener("dragover", (event) => {
	const zone = dropZone(event);
	if (!zone) return;
	event.preventDefault();
	event.dataTransfer.dropEffect = "move";
	for (const other of document.querySelectorAll("[data-drop-over]")) {
		if (other !== zone) delete other.dataset.dropOver;
	}
	zone.dataset.dropOver = "true";
});

document.addEventListener("drop", async (event) => {
	const zone = dropZone(event);
	if (!zone) return;
	event.preventDefault();
	const id = dragged.id;
	delete zone.dataset.dropOver;
	try {
		await post(zone.dataset.drop.replace(":id", id), params(zone));
	} finally {
		await refresh();
	}
});
