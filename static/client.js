(function() {
  let ws;
  let state = { ports: [], mappings: [] };

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(proto + '//' + location.host + '/ws');

    ws.onopen = function() {
      console.log('Portgate WS connected');
    };

    ws.onmessage = function(e) {
      const msg = JSON.parse(e.data);
      if (msg.type === 'update') {
        state.ports = msg.data.ports || [];
        state.mappings = msg.data.mappings || [];
        render();
      }
    };

    ws.onclose = function() {
      console.log('Portgate WS disconnected, reconnecting...');
      setTimeout(connect, 2000);
    };
  }

  function render() {
    renderPorts();
    renderMappings();
  }

  function renderPorts() {
    const el = document.getElementById('ports');
    if (!state.ports.length) {
      el.innerHTML = '<div class="empty">No ports discovered yet...</div>';
      return;
    }

    el.innerHTML = state.ports.map(function(p) {
      const detail = [p.serviceName, p.title].filter(Boolean).join(' — ');
      const mapped = state.mappings.find(function(m) { return m.targetPort === p.port; });
      return '<div class="port-item">' +
        '<div class="port-info">' +
          '<span class="status-dot ' + (p.healthy ? 'online' : 'offline') + '"></span>' +
          '<span class="port-number">:' + p.port + '</span>' +
          '<span class="port-detail">' + escapeHtml(detail) + '</span>' +
        '</div>' +
        (mapped
          ? '<span class="mapping-domain">' + escapeHtml(mapped.domain) + '.localhost</span>'
          : '<div class="map-form">' +
              '<input type="text" placeholder="subdomain" id="map-input-' + p.port + '" ' +
                'onkeydown="if(event.key===\'Enter\')mapDomain(' + p.port + ')">' +
              '<button class="btn btn-primary" onclick="mapDomain(' + p.port + ')">Map</button>' +
            '</div>'
        ) +
      '</div>';
    }).join('');
  }

  function renderMappings() {
    const el = document.getElementById('mappings');
    if (!state.mappings.length) {
      el.innerHTML = '<div class="empty">No domain mappings configured</div>';
      return;
    }

    el.innerHTML = state.mappings.map(function(m) {
      const port = state.ports.find(function(p) { return p.port === m.targetPort; });
      const online = port && port.healthy;
      return '<div class="mapping-item">' +
        '<div class="mapping-info">' +
          '<span class="status-dot ' + (online ? 'online' : 'offline') + '"></span>' +
          '<span class="mapping-domain">' + escapeHtml(m.domain) + '.localhost</span>' +
          '<span class="mapping-target">→ :' + m.targetPort + '</span>' +
        '</div>' +
        '<button class="btn btn-danger" onclick="removeMapping(\'' + escapeHtml(m.domain) + '\')">Remove</button>' +
      '</div>';
    }).join('');
  }

  window.mapDomain = function(port) {
    const input = document.getElementById('map-input-' + port);
    const domain = input.value.trim().toLowerCase();
    if (!domain) return;

    fetch('/api/mappings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain: domain, port: port })
    }).then(function(r) {
      if (!r.ok) r.text().then(function(t) { alert('Error: ' + t); });
    });
  };

  window.removeMapping = function(domain) {
    fetch('/api/mappings?domain=' + encodeURIComponent(domain), {
      method: 'DELETE'
    });
  };

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  connect();
})();
