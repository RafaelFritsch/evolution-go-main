(function () {
  const params = new URLSearchParams(window.location.search);
  const instanceName = (params.get("instance") || "").trim();
  const instanceId = (params.get("instanceId") || "").trim();

  const defaults = {
    enabled: false,
    url: "",
    accountId: "",
    token: "",
    signMsg: false,
    signDelimiter: "\n",
    reopenConversation: false,
    conversationPending: false,
    nameInbox: instanceName,
    mergeBrazilContacts: false,
    importContacts: false,
    importMessages: false,
    daysLimitImportMessages: 7,
    autoCreate: false,
    organization: "",
    logo: "",
    ignoreJids: [],
  };

  const form = document.getElementById("chatwootForm");
  const feedback = document.getElementById("feedback");
  const subtitle = document.getElementById("pageSubtitle");
  const backToSettings = document.getElementById("backToSettings");
  const saveButton = document.getElementById("saveButton");
  const removeButton = document.getElementById("removeButton");

  const auth = loadAuth();
  let currentPayload = { ...defaults };

  if (instanceId) {
    backToSettings.href = `/manager/instances/${encodeURIComponent(instanceId)}/settings`;
  }

  if (!instanceName) {
    subtitle.textContent = "Nenhuma instancia foi informada na URL.";
    form.classList.add("hidden");
    showFeedback("error", "Abra esta pagina pelo Manager para carregar a instancia correta.");
    return;
  }

  subtitle.textContent = `Instancia ${instanceName} conectada ao fluxo de configuracao do Chatwoot.`;
  setFormValues(defaults);

  if (!auth.apiKey) {
    showFeedback("error", "Nao foi encontrada uma sessao ativa do Manager. Faca login novamente para salvar a integracao.");
  } else {
    loadConfig();
  }

  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    await saveConfig(true);
  });

  removeButton.addEventListener("click", async function () {
    if (!window.confirm(`Desabilitar a integracao Chatwoot da instancia ${instanceName}?`)) {
      return;
    }
    await saveConfig(false);
  });

  async function loadConfig() {
    setLoading(true);
    try {
      const response = await apiRequest(`/chatwoot/find/${encodeURIComponent(instanceName)}`, {
        method: "GET",
      });

      if (response.status === 404) {
        currentPayload = { ...defaults, nameInbox: instanceName };
        setFormValues(currentPayload);
        showFeedback("info", "Nenhuma configuracao salva foi encontrada. Preencha o formulario para iniciar a integracao.");
        return;
      }

      if (!response.ok) {
        throw new Error(await response.text());
      }

      const payload = await response.json();
      currentPayload = normalizePayload(payload);
      setFormValues(currentPayload);
      showFeedback("success", "Configuracao carregada com sucesso.");
    } catch (error) {
      showFeedback("error", formatError(error, "Falha ao carregar a configuracao do Chatwoot."));
    } finally {
      setLoading(false);
    }
  }

  async function saveConfig(enableIntegration) {
    if (!auth.apiKey) {
      showFeedback("error", "Sessao do Manager ausente. Recarregue a pagina e faca login novamente.");
      return;
    }

    const payload = collectPayload();
    payload.enabled = enableIntegration;

    if (!payload.token) {
      showFeedback("error", "O token do Chatwoot e obrigatorio para salvar ou desabilitar a integracao.");
      return;
    }

    setLoading(true);
    try {
      const response = await apiRequest(`/chatwoot/set/${encodeURIComponent(instanceName)}`, {
        method: "POST",
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error(await response.text());
      }

      const saved = await response.json();
      currentPayload = normalizePayload({ ...payload, ...saved, enabled: payload.enabled });
      setFormValues(currentPayload);
      showFeedback(
        "success",
        enableIntegration
          ? "Configuracao salva com sucesso."
          : "Integracao desabilitada com sucesso. Os parametros foram mantidos para futura reativacao."
      );
    } catch (error) {
      showFeedback("error", formatError(error, "Falha ao salvar a configuracao do Chatwoot."));
    } finally {
      setLoading(false);
    }
  }

  function collectPayload() {
    return {
      enabled: readChecked("enabled"),
      url: readValue("url"),
      accountId: readValue("accountId"),
      token: readValue("token"),
      signMsg: readChecked("signMsg"),
      signDelimiter: readValue("signDelimiter") || "\n",
      reopenConversation: readChecked("reopenConversation"),
      conversationPending: readChecked("conversationPending"),
      nameInbox: readValue("nameInbox") || instanceName,
      mergeBrazilContacts: readChecked("mergeBrazilContacts"),
      importContacts: readChecked("importContacts"),
      importMessages: readChecked("importMessages"),
      daysLimitImportMessages: Number(readValue("daysLimitImportMessages") || 7),
      autoCreate: readChecked("autoCreate"),
      organization: readValue("organization"),
      logo: readValue("logo"),
      ignoreJids: readValue("ignoreJids")
        .split(/\n|,/)
        .map((value) => value.trim())
        .filter(Boolean),
    };
  }

  function normalizePayload(payload) {
    return {
      ...defaults,
      ...payload,
      nameInbox: payload.nameInbox || instanceName,
      signDelimiter: payload.signDelimiter || "\n",
      daysLimitImportMessages: Number(payload.daysLimitImportMessages || 7),
      ignoreJids: Array.isArray(payload.ignoreJids) ? payload.ignoreJids : [],
    };
  }

  function setFormValues(payload) {
    writeValue("url", payload.url || "");
    writeValue("accountId", payload.accountId || "");
    writeValue("token", payload.token || "");
    writeValue("nameInbox", payload.nameInbox || instanceName);
    writeValue("organization", payload.organization || "");
    writeValue("logo", payload.logo || "");
    writeValue("daysLimitImportMessages", String(payload.daysLimitImportMessages || 7));
    writeValue("signDelimiter", payload.signDelimiter || "\n");
    writeValue("ignoreJids", (payload.ignoreJids || []).join("\n"));

    writeChecked("enabled", !!payload.enabled);
    writeChecked("signMsg", !!payload.signMsg);
    writeChecked("reopenConversation", !!payload.reopenConversation);
    writeChecked("conversationPending", !!payload.conversationPending);
    writeChecked("mergeBrazilContacts", !!payload.mergeBrazilContacts);
    writeChecked("importContacts", !!payload.importContacts);
    writeChecked("importMessages", !!payload.importMessages);
    writeChecked("autoCreate", !!payload.autoCreate);
  }

  function readValue(id) {
    const element = document.getElementById(id);
    return element ? element.value.trim() : "";
  }

  function writeValue(id, value) {
    const element = document.getElementById(id);
    if (element) {
      element.value = value;
    }
  }

  function readChecked(id) {
    const element = document.getElementById(id);
    return !!(element && element.checked);
  }

  function writeChecked(id, value) {
    const element = document.getElementById(id);
    if (element) {
      element.checked = value;
    }
  }

  function setLoading(loading) {
    saveButton.disabled = loading;
    removeButton.disabled = loading;
    saveButton.textContent = loading ? "Salvando..." : "Salvar";
    removeButton.textContent = loading ? "Aguarde..." : "Remover integracao";
  }

  async function apiRequest(path, options) {
    const baseUrl = (auth.apiUrl || window.location.origin).replace(/\/$/, "");
    const headers = new Headers(options && options.headers ? options.headers : {});
    headers.set("Content-Type", "application/json");
    if (auth.apiKey) {
      headers.set("apikey", auth.apiKey);
    }
    return fetch(`${baseUrl}${path}`, {
      ...options,
      headers,
    });
  }

  function loadAuth() {
    try {
      const raw = window.localStorage.getItem("evolution-auth");
      if (!raw) {
        return { apiUrl: window.location.origin, apiKey: "" };
      }
      const parsed = JSON.parse(raw);
      const state = parsed.state || parsed;
      return {
        apiUrl: state.apiUrl || window.location.origin,
        apiKey: state.apiKey || "",
      };
    } catch (error) {
      return { apiUrl: window.location.origin, apiKey: "" };
    }
  }

  function showFeedback(tone, message) {
    feedback.dataset.tone = tone;
    feedback.textContent = message;
    feedback.classList.remove("hidden");
  }

  function formatError(error, fallback) {
    if (!error) {
      return fallback;
    }
    if (typeof error.message === "string" && error.message.trim()) {
      return `${fallback} ${error.message}`;
    }
    return fallback;
  }
})();
