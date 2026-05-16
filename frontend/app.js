const state = {
  apiUrl: localStorage.getItem("eventbookerApiUrl") || "http://localhost:8080",
  events: [],
  selectedTab: "user",
  lastBooking: null,
};

const els = {
  apiUrl: document.querySelector("#apiUrl"),
  saveApi: document.querySelector("#saveApi"),
  tabs: document.querySelectorAll(".tab"),
  userTab: document.querySelector("#userTab"),
  adminTab: document.querySelector("#adminTab"),
  userEvents: document.querySelector("#userEvents"),
  adminEvents: document.querySelector("#adminEvents"),
  refreshUser: document.querySelector("#refreshUser"),
  refreshAdmin: document.querySelector("#refreshAdmin"),
  createEventForm: document.querySelector("#createEventForm"),
  bookingForm: document.querySelector("#bookingForm"),
  userName: document.querySelector("#userName"),
  userEmail: document.querySelector("#userEmail"),
  lastBooking: document.querySelector("#lastBooking"),
  toast: document.querySelector("#toast"),
};

els.apiUrl.value = state.apiUrl;

function endpoint(path) {
  return `${state.apiUrl.replace(/\/$/, "")}${path}`;
}

async function request(path, options = {}) {
  const res = await fetch(endpoint(path), {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });

  const payload = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(payload.error || `HTTP ${res.status}`);
  }

  return payload;
}

async function loadEvents() {
  const events = await request("/events");
  state.events = await Promise.all(
    events.map((item) => request(`/events/${item.event.id}`).catch(() => item)),
  );
  render();
}

function render() {
  els.userEvents.innerHTML = state.events.map(renderUserEvent).join("");
  els.adminEvents.innerHTML = state.events.map(renderAdminEvent).join("");
  renderLastBooking();
}

function renderUserEvent(item) {
  const event = item.event;
  const disabled = item.freeSeats <= 0 ? "disabled" : "";
  return `
    <article class="event-card">
      <div class="event-title">
        <div>
          <h3>${escapeHTML(event.title)}</h3>
          <div class="event-date">${formatDate(event.eventDate)}</div>
        </div>
        <span class="badge ${item.freeSeats > 0 ? "ok" : "warn"}">${item.freeSeats} свободно</span>
      </div>
      <div class="metrics">
        ${metric(item.freeSeats, "свободно")}
        ${metric(item.pendingCount, "ожидают")}
        ${metric(item.confirmedCount, "подтверждены")}
      </div>
      <div class="booking-meta">Дедлайн оплаты: ${formatTTL(event.bookingTtl)}</div>
      <div class="actions">
        <button type="button" ${disabled} data-action="book" data-id="${event.id}">Забронировать</button>
        <button class="secondary" type="button" data-action="confirm" data-id="${event.id}">Подтвердить</button>
      </div>
    </article>
  `;
}

function renderAdminEvent(item) {
  const event = item.event;
  const bookings = item.bookings && item.bookings.length > 0
    ? item.bookings.map(renderBookingRow).join("")
    : `<tr><td colspan="5">Активных броней нет</td></tr>`;

  return `
    <article class="event-row">
      <div class="event-title">
        <div>
          <h3>${escapeHTML(event.title)}</h3>
          <div class="event-date">${formatDate(event.eventDate)} · мест: ${event.capacity} · TTL: ${formatTTL(event.bookingTtl)}</div>
        </div>
        <span class="badge ${item.freeSeats > 0 ? "ok" : "warn"}">${item.freeSeats} свободно</span>
      </div>
      <table class="bookings">
        <thead>
          <tr>
            <th>ID</th>
            <th>Пользователь</th>
            <th>Email</th>
            <th>Статус</th>
            <th>Истекает</th>
          </tr>
        </thead>
        <tbody>${bookings}</tbody>
      </table>
    </article>
  `;
}

function renderBookingRow(booking) {
  const badgeClass = booking.status === "confirmed" ? "ok" : "warn";
  return `
    <tr>
      <td>${booking.id}</td>
      <td>${escapeHTML(booking.userName)}</td>
      <td>${escapeHTML(booking.userEmail)}</td>
      <td><span class="badge ${badgeClass}">${translateStatus(booking.status)}</span></td>
      <td>${formatDate(booking.expiresAt)}</td>
    </tr>
  `;
}

function metric(value, label) {
  return `<div class="metric"><strong>${value}</strong><span>${label}</span></div>`;
}

function renderLastBooking() {
  if (!state.lastBooking) {
    els.lastBooking.textContent = "Бронь пока не выбрана";
    return;
  }

  els.lastBooking.textContent = `Последняя бронь #${state.lastBooking.id}, истекает ${formatDate(state.lastBooking.expiresAt)}`;
}

async function createEvent(event) {
  event.preventDefault();
  const data = new FormData(els.createEventForm);
  const payload = {
    title: data.get("title"),
    eventDate: data.get("eventDate"),
    capacity: Number(data.get("capacity")),
    bookingTtlMinutes: Number(data.get("bookingTtlMinutes")),
    requiresPayment: data.get("requiresPayment") === "on",
  };

  await request("/events", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  els.createEventForm.reset();
  els.createEventForm.elements.capacity.value = 10;
  els.createEventForm.elements.bookingTtlMinutes.value = 2;
  els.createEventForm.elements.requiresPayment.checked = true;
  notify("Мероприятие создано");
  await loadEvents();
}

async function bookEvent(eventID) {
  const payload = readUserPayload();
  const booking = await request(`/events/${eventID}/book`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  state.lastBooking = booking;
  notify(`Бронь #${booking.id} создана`);
  await loadEvents();
}

async function confirmBooking(eventID) {
  const payload = { userEmail: els.userEmail.value.trim() };
  if (!payload.userEmail) {
    notify("Введите email для подтверждения");
    return;
  }

  const booking = await request(`/events/${eventID}/confirm`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  state.lastBooking = booking;
  notify(`Бронь #${booking.id} подтверждена`);
  await loadEvents();
}

function readUserPayload() {
  const userName = els.userName.value.trim();
  const userEmail = els.userEmail.value.trim();
  if (!userName || !userEmail) {
    throw new Error("Введите имя и email");
  }

  return { userName, userEmail };
}

function switchTab(tab) {
  state.selectedTab = tab;
  els.tabs.forEach((item) => item.classList.toggle("is-active", item.dataset.tab === tab));
  els.userTab.classList.toggle("is-hidden", tab !== "user");
  els.adminTab.classList.toggle("is-hidden", tab !== "admin");
}

function notify(message) {
  els.toast.textContent = message;
  els.toast.classList.add("is-visible");
  window.clearTimeout(notify.timeout);
  notify.timeout = window.setTimeout(() => els.toast.classList.remove("is-visible"), 2600);
}

function formatDate(value) {
  return new Intl.DateTimeFormat("ru-RU", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatTTL(value) {
  if (typeof value === "number") {
    return `${Math.round(value / 60000000000)} мин`;
  }
  if (typeof value === "string") {
    return value;
  }
  return "по умолчанию";
}

function translateStatus(status) {
  const map = {
    pending: "ожидает",
    confirmed: "подтверждена",
    canceled: "отменена",
  };
  return map[status] || status;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

els.saveApi.addEventListener("click", async () => {
  state.apiUrl = els.apiUrl.value.trim() || "http://localhost:8080";
  localStorage.setItem("eventbookerApiUrl", state.apiUrl);
  notify("API адрес сохранен");
  await loadEvents().catch((err) => notify(err.message));
});

els.tabs.forEach((tab) => {
  tab.addEventListener("click", () => switchTab(tab.dataset.tab));
});

els.refreshUser.addEventListener("click", () => loadEvents().catch((err) => notify(err.message)));
els.refreshAdmin.addEventListener("click", () => loadEvents().catch((err) => notify(err.message)));
els.createEventForm.addEventListener("submit", (event) => createEvent(event).catch((err) => notify(err.message)));

document.addEventListener("click", (event) => {
  const button = event.target.closest("[data-action]");
  if (!button) {
    return;
  }

  const eventID = button.dataset.id;
  if (button.dataset.action === "book") {
    bookEvent(eventID).catch((err) => notify(err.message));
  }
  if (button.dataset.action === "confirm") {
    confirmBooking(eventID).catch((err) => notify(err.message));
  }
});

loadEvents().catch((err) => notify(err.message));
window.setInterval(() => loadEvents().catch(() => {}), 5000);
