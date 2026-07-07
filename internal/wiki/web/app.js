// ==========================================================================
// WIKI PREMIUM INTERACTIVE JAVASCRIPT
// ==========================================================================

document.addEventListener('DOMContentLoaded', () => {
    // STATE MANAGEMENT
    let activePath = null;
    let activeDocPath = null;
    let activeMode = 'wiki'; // 'wiki' or 'docs'
    let chatHistory = [];
    const expandedFolders = new Set(['01-global', '04-systems', '05-decisions']); // expand by default

    // DOCUMENTATION CHAPTERS LIST
    const docChapters = [
        { name: "✨ 01. Introdução", path: "docs/01-introducao.md" },
        { name: "🔍 02. Workspace & Scanner", path: "docs/02-workspace-scan.md" },
        { name: "🚀 03. Ciclo de Vida da Sessão", path: "docs/03-session-lifecycle.md" },
        { name: "📦 04. Context Builder", path: "docs/04-context-builder.md" },
        { name: "🏃 05. Execução & Auditoria", path: "docs/05-execution-harness.md" },
        { name: "🚦 06. Quality Gates", path: "docs/06-quality-gates.md" },
        { name: "⚡ 07. Diff Summarizer", path: "docs/07-diff-summarizer.md" },
        { name: "📑 08. Relatório da Sessão", path: "docs/08-session-report.md" },
        { name: "📥 09. LLM Wiki (Karpathy)", path: "docs/09-llm-wiki.md" },
        { name: "🛠️ 10. Utilitários do Vault", path: "docs/10-vault-utils.md" },
        { name: "🔌 11. Integração OpenCode", path: "docs/11-opencode-integration.md" },
        { name: "📋 12. Workflows & Tasks", path: "docs/12-workflows-tasks.md" },
        { name: "🛠️ 13. Exemplo de Desenvolvimento", path: "docs/13-desenvolvimento-passo-a-passo.md" },
        { name: "🖥️ 14. Interface TUI", path: "docs/14-interface-tui.md" }
    ];

    // DOM ELEMENTS
    const vaultTree = document.getElementById('vault-tree');
    const docViewer = document.getElementById('doc-viewer');
    const chatMessages = document.getElementById('chat-messages');
    const chatForm = document.getElementById('chat-form');
    const chatInput = document.getElementById('chat-input');
    const btnClearChat = document.getElementById('btn-clear-chat');
    const btnRefreshTree = document.getElementById('btn-refresh-tree');

    // NAVIGATION TABS DOM
    const btnNavWiki = document.getElementById('btn-nav-wiki');
    const btnNavDocs = document.getElementById('btn-nav-docs');
    const btnNavGraph = document.getElementById('btn-nav-graph');
    const sidebarTitleText = document.getElementById('sidebar-title-text');
    const graphContainer = document.getElementById('graph-container');
    let graphInstance = null; // Store the force-graph instance

    // SEARCH MODAL DOM
    const searchTriggerBtn = document.getElementById('search-trigger-btn');
    const searchModal = document.getElementById('search-modal');
    const searchInput = document.getElementById('search-input');
    const searchResults = document.getElementById('search-results');
    const btnCloseSearch = document.getElementById('btn-close-search');

    // INITIALIZATION
    loadDirectoryTree();
    setupNavigation();
    initMermaid();

    function initMermaid() {
        if (typeof mermaid !== 'undefined') {
            mermaid.initialize({
                startOnLoad: false,
                theme: 'dark',
                securityLevel: 'loose'
            });
        }
    }

    // 1. DIRECTORY TREE LOADER
    async function loadDirectoryTree() {
        try {
            const res = await fetch('/api/wiki/docs');
            if (!res.ok) throw new Error('Falha ao carregar diretórios');
            const data = await res.json();
            renderDirectoryTree(data);
        } catch (err) {
            console.error(err);
            vaultTree.innerHTML = `<div class="error-state">Erro: ${err.message}</div>`;
        }
    }

    function renderDirectoryTree(nodes) {
        vaultTree.innerHTML = '';
        if (!nodes || nodes.length === 0) {
            vaultTree.innerHTML = '<div class="empty-state">Nenhuma pasta encontrada.</div>';
            return;
        }
        
        // Sort nodes so folders come first
        nodes.sort((a, b) => {
            if (a.is_dir === b.is_dir) return a.name.localeCompare(b.name);
            return a.is_dir ? -1 : 1;
        });

        nodes.forEach(node => {
            vaultTree.appendChild(createTreeNode(node));
        });
    }

    function createTreeNode(node) {
        if (node.is_dir) {
            const folderDiv = document.createElement('div');
            folderDiv.className = 'tree-folder';
            
            const isExpanded = expandedFolders.has(node.name);

            const header = document.createElement('div');
            header.className = 'folder-header';
            header.innerHTML = `
                <span class="folder-icon" style="transform: ${isExpanded ? 'rotate(90deg)' : 'none'}; display: inline-block;">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"></polyline></svg>
                </span>
                <span>📁 ${node.name}</span>
            `;

            const childrenContainer = document.createElement('div');
            childrenContainer.className = 'folder-children';
            childrenContainer.style.display = isExpanded ? 'block' : 'none';

            // Add toggle event
            header.addEventListener('click', () => {
                const expanded = childrenContainer.style.display === 'block';
                childrenContainer.style.display = expanded ? 'none' : 'block';
                const icon = header.querySelector('.folder-icon');
                icon.style.transform = expanded ? 'none' : 'rotate(90deg)';
                
                if (expanded) {
                    expandedFolders.delete(node.name);
                } else {
                    expandedFolders.add(node.name);
                }
            });

            folderDiv.appendChild(header);

            if (node.children && node.children.length > 0) {
                // Sort children
                node.children.sort((a, b) => {
                    if (a.is_dir === b.is_dir) return a.name.localeCompare(b.name);
                    return a.is_dir ? -1 : 1;
                });

                node.children.forEach(child => {
                    childrenContainer.appendChild(createTreeNode(child));
                });
            } else {
                const empty = document.createElement('div');
                empty.className = 'tree-file';
                empty.style.color = 'var(--text-muted)';
                empty.style.fontStyle = 'italic';
                empty.innerText = '(Vazia)';
                childrenContainer.appendChild(empty);
            }

            folderDiv.appendChild(childrenContainer);
            return folderDiv;
        } else {
            const fileLink = document.createElement('a');
            fileLink.className = 'tree-file';
            if (activePath === node.path) {
                fileLink.classList.add('active');
            }
            fileLink.href = `#${node.path}`;
            fileLink.innerHTML = `<span>📄 ${node.name}</span>`;
            
            fileLink.addEventListener('click', (e) => {
                e.preventDefault();
                selectDocument(node.path);
            });

            return fileLink;
        }
    }

    // 2. DOCUMENT SELECTOR & RENDERER
    async function selectDocument(path) {
        if (activeMode === 'graph') {
            activeMode = 'wiki';
            btnNavWiki.classList.add('active');
            btnNavGraph.classList.remove('active');
            sidebarTitleText.innerText = 'Vault Explorer';
            btnRefreshTree.style.display = 'flex';
            loadDirectoryTree();
            
            document.querySelector('.content-area').classList.remove('graph-mode');
            graphContainer.style.display = 'none';
            docViewer.style.display = 'block';
        }
        
        if (activeMode === 'docs') {
            activeDocPath = path;
        } else {
            activePath = path;
        }
        
        // Update active class in sidebar tree
        document.querySelectorAll('.tree-file').forEach(el => {
            el.classList.remove('active');
        });
        const currentLinks = document.querySelectorAll(`a[href="#${path}"]`);
        currentLinks.forEach(link => link.classList.add('active'));

        docViewer.innerHTML = '<div class="loading-state">Carregando documento...</div>';

        try {
            let content = '';
            if (activeMode === 'docs') {
                const res = await fetch(path);
                if (!res.ok) throw new Error('Não foi possível ler o arquivo');
                content = await res.text();
            } else {
                const res = await fetch(`/api/wiki/doc?path=${encodeURIComponent(path)}`);
                if (!res.ok) throw new Error('Não foi possível ler o arquivo');
                const data = await res.json();
                content = data.content;
            }
            
            renderMarkdown(content, path);
        } catch (err) {
            console.error(err);
            docViewer.innerHTML = `<div class="error-state">Erro ao abrir documento: ${err.message}</div>`;
        }
    }

    function renderMarkdown(rawText, path) {
        const { metadata, markdown } = parseFrontmatter(rawText);

        let html = '';

        // Render Frontmatter Box if it exists
        if (metadata) {
            html += `<div class="frontmatter-box">`;
            if (metadata.title) {
                html += `<div class="frontmatter-item">
                    <span class="frontmatter-label">Título</span>
                    <span class="frontmatter-value">${metadata.title}</span>
                </div>`;
            }
            if (metadata.status) {
                html += `<div class="frontmatter-item">
                    <span class="frontmatter-label">Status</span>
                    <span class="frontmatter-value">${metadata.status}</span>
                </div>`;
            }
            if (metadata.updated_at) {
                html += `<div class="frontmatter-item">
                    <span class="frontmatter-label">Atualizado</span>
                    <span class="frontmatter-value">${metadata.updated_at}</span>
                </div>`;
            }
            if (metadata.tags && metadata.tags.length > 0) {
                html += `<div class="frontmatter-item">
                    <span class="frontmatter-label">Tags</span>
                    <span class="frontmatter-value">`;
                metadata.tags.forEach(tag => {
                    html += `<span class="tag-badge">${tag}</span>`;
                });
                html += `</span></div>`;
            }
            html += `</div>`;
        }

        // Parse markdown content using Marked.js with preprocessed alerts
        const processedMarkdown = preprocessMarkdown(markdown);
        html += marked.parse(processedMarkdown);

        docViewer.innerHTML = html;
        docViewer.scrollTop = 0;

        // Render Mermaid Diagrams if present
        renderMermaidDiagrams();

        // Apply copy buttons to code blocks
        setupCopyButtons();

        // Hijack links within markdown to enable SPA navigation if they are internal
        docViewer.querySelectorAll('a').forEach(a => {
            const href = a.getAttribute('href');
            if (href && !href.startsWith('http') && !href.startsWith('#') && href.endsWith('.md')) {
                a.addEventListener('click', (e) => {
                    e.preventDefault();
                    
                    // Resolve relative path
                    const currentDir = path.substring(0, path.lastIndexOf('/'));
                    const resolvedPath = resolveRelativePath(currentDir, href);
                    selectDocument(resolvedPath);
                });
            }
        });
    }

    // Helper to preprocess markdown alerts
    function preprocessMarkdown(markdown) {
        // Matches blockquotes of type > [!NOTE] etc.
        const alertRegex = />\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*\n((?:>\s*.*\n?)*)/gi;
        return markdown.replace(alertRegex, (match, type, content) => {
            const cleanContent = content.split('\n')
                                        .map(line => line.replace(/^\s*>\s?/, ''))
                                        .join('\n');
            
            let emoji = '📖';
            let label = 'Nota';
            const uType = type.toUpperCase();
            if (uType === 'TIP') { emoji = '💡'; label = 'Dica'; }
            else if (uType === 'IMPORTANT') { emoji = '⚠️'; label = 'Importante'; }
            else if (uType === 'WARNING') { emoji = '🔸'; label = 'Atenção'; }
            else if (uType === 'CAUTION') { emoji = '🚨'; label = 'Cuidado'; }
            
            return `<div class="alert-box alert-${uType.toLowerCase()}"><div class="alert-title">${emoji} ${label}</div>${marked.parse(cleanContent)}</div>`;
        });
    }

    // Render Mermaid Diagrams helper
    function renderMermaidDiagrams() {
        if (typeof mermaid === 'undefined') return;
        
        const mermaidBlocks = docViewer.querySelectorAll('pre code.language-mermaid');
        if (mermaidBlocks.length === 0) return;

        mermaidBlocks.forEach(code => {
            const pre = code.parentNode;
            const div = document.createElement('div');
            div.className = 'mermaid';
            div.textContent = code.textContent;
            pre.parentNode.replaceChild(div, pre);
        });

        try {
            if (typeof mermaid.run === 'function') {
                mermaid.run();
            } else if (typeof mermaid.init === 'function') {
                mermaid.init(undefined, docViewer.querySelectorAll('.mermaid'));
            }
        } catch (err) {
            console.error('Erro ao renderizar Mermaid:', err);
        }
    }

    // Copy to clipboard setup
    function setupCopyButtons() {
        docViewer.querySelectorAll('pre').forEach(pre => {
            if (pre.parentNode.classList.contains('code-block-container')) return;

            const container = document.createElement('div');
            container.className = 'code-block-container';
            pre.parentNode.insertBefore(container, pre);
            container.appendChild(pre);

            const copyBtn = document.createElement('button');
            copyBtn.className = 'copy-code-btn';
            copyBtn.innerText = 'Copiar';
            container.appendChild(copyBtn);

            copyBtn.addEventListener('click', async () => {
                const codeText = pre.innerText;
                try {
                    await navigator.clipboard.writeText(codeText);
                    copyBtn.innerText = 'Copiado!';
                    copyBtn.classList.add('copied');
                    setTimeout(() => {
                        copyBtn.innerText = 'Copiar';
                        copyBtn.classList.remove('copied');
                    }, 2000);
                } catch (err) {
                    console.error('Falha ao copiar código:', err);
                }
            });
        });
    }

    // Navigation setup
    function setupNavigation() {
        btnNavWiki.addEventListener('click', () => {
            if (activeMode === 'wiki') return;
            activeMode = 'wiki';
            btnNavWiki.classList.add('active');
            btnNavDocs.classList.remove('active');
            btnNavGraph.classList.remove('active');
            sidebarTitleText.innerText = 'Vault Explorer';
            btnRefreshTree.style.display = 'flex';
            loadDirectoryTree();
            
            document.querySelector('.content-area').classList.remove('graph-mode');
            graphContainer.style.display = 'none';
            docViewer.style.display = 'block';
            
            if (activePath) {
                selectDocument(activePath);
            } else {
                showWelcomeScreen();
            }
        });

        btnNavDocs.addEventListener('click', () => {
            if (activeMode === 'docs') return;
            activeMode = 'docs';
            btnNavDocs.classList.add('active');
            btnNavWiki.classList.remove('active');
            btnNavGraph.classList.remove('active');
            sidebarTitleText.innerText = 'Documentação';
            btnRefreshTree.style.display = 'none';
            renderDocsMenu();
            
            document.querySelector('.content-area').classList.remove('graph-mode');
            graphContainer.style.display = 'none';
            docViewer.style.display = 'block';
            
            if (activeDocPath) {
                selectDocument(activeDocPath);
            } else if (docChapters.length > 0) {
                selectDocument(docChapters[0].path);
            }
        });

        btnNavGraph.addEventListener('click', () => {
            if (activeMode === 'graph') return;
            activeMode = 'graph';
            btnNavGraph.classList.add('active');
            btnNavWiki.classList.remove('active');
            btnNavDocs.classList.remove('active');
            sidebarTitleText.innerText = 'Vault Explorer';
            btnRefreshTree.style.display = 'flex';
            loadDirectoryTree();
            
            document.querySelector('.content-area').classList.add('graph-mode');
            docViewer.style.display = 'none';
            graphContainer.style.display = 'block';
            
            renderGraph();
        });
    }

    function renderDocsMenu() {
        vaultTree.innerHTML = '';
        docChapters.forEach(chapter => {
            const fileLink = document.createElement('a');
            fileLink.className = 'tree-file';
            if (activeDocPath === chapter.path) {
                fileLink.classList.add('active');
            }
            fileLink.href = `#${chapter.path}`;
            fileLink.innerHTML = `<span>${chapter.name}</span>`;
            
            fileLink.addEventListener('click', (e) => {
                e.preventDefault();
                selectDocument(chapter.path);
            });
            vaultTree.appendChild(fileLink);
        });
    }

    function showWelcomeScreen() {
        docViewer.innerHTML = `
            <div class="welcome-screen">
                <div class="welcome-icon">📖</div>
                <h1>Bem-vindo à LLM Wiki do kv</h1>
                <p>Selecione um documento no painel esquerdo ou utilize a pesquisa rápida para começar.</p>
                <div class="welcome-features">
                    <div class="feature-card">
                        <h4>Compilação Automática</h4>
                        <p>Notas cruas da inbox são compiladas e organizadas por IA no cofre.</p>
                    </div>
                    <div class="feature-card">
                        <h4>Links Cruzados</h4>
                        <p>Menções a termos de arquitetura e ADRs são vinculadas automaticamente.</p>
                    </div>
                    <div class="feature-card">
                        <h4>Assistente RAG</h4>
                        <p>Tire dúvidas técnicas diretamente com a IA no chat lateral.</p>
                    </div>
                </div>
            </div>
        `;
    }

    // Helper to resolve paths like "04-systems/../05-decisions/adr.md" to "05-decisions/adr.md"
    function resolveRelativePath(baseDir, relPath) {
        const parts = (baseDir + '/' + relPath).split('/');
        const stack = [];
        for (const part of parts) {
            if (part === '.' || part === '') continue;
            if (part === '..') {
                if (stack.length > 0) stack.pop();
            } else {
                stack.push(part);
            }
        }
        return stack.join('/');
    }

    function parseFrontmatter(rawText) {
        if (rawText.startsWith('---')) {
            const parts = rawText.split('---');
            if (parts.length >= 3) {
                const frontmatterRaw = parts[1];
                const markdown = parts.slice(2).join('---');

                // simple yaml parser
                const metadata = {};
                frontmatterRaw.split('\n').forEach(line => {
                    const colonIdx = line.indexOf(':');
                    if (colonIdx !== -1) {
                        const key = line.substring(0, colonIdx).trim();
                        let val = line.substring(colonIdx + 1).trim();
                        
                        if (val.startsWith('[') && val.endsWith(']')) {
                            val = val.substring(1, val.length - 1)
                                     .split(',')
                                     .map(t => t.trim().replace(/^["']|["']$/g, ''))
                                     .filter(t => t !== '');
                        } else {
                            val = val.replace(/^["']|["']$/g, '');
                        }
                        metadata[key] = val;
                    }
                });

                return { metadata, markdown };
            }
        }
        return { metadata: null, markdown: rawText };
    }

    // 3. AI CHAT ASSISTANT
    chatForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const text = chatInput.value.trim();
        if (!text) return;

        chatInput.value = '';
        chatInput.style.height = 'auto';

        // Add user message
        appendChatMessage('user', text);
        chatHistory.push({ role: 'user', text: text });

        // Add model loading message
        const loadingDiv = appendChatMessage('system', 'Processando pergunta...');

        try {
            const res = await fetch('/api/wiki/chat', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ query: text, history: chatHistory })
            });

            if (!res.ok) throw new Error('Erro na requisição RAG');
            const data = await res.json();
            
            loadingDiv.remove(); // Remove loading state
            appendChatMessage('model', data.answer);
            chatHistory.push({ role: 'model', text: data.answer });
        } catch (err) {
            console.error(err);
            loadingDiv.innerText = `Erro: ${err.message}`;
            loadingDiv.className = 'chat-message model-msg';
            loadingDiv.style.color = '#ef4444';
        }
    });

    function appendChatMessage(role, text) {
        const div = document.createElement('div');
        if (role === 'user') {
            div.className = 'chat-message user-msg';
            div.innerText = text;
        } else if (role === 'model') {
            div.className = 'chat-message model-msg';
            div.innerHTML = marked.parse(text);
        } else {
            div.className = 'chat-message system-status';
            div.innerText = text;
        }
        chatMessages.appendChild(div);
        chatMessages.scrollTop = chatMessages.scrollHeight;
        return div;
    }

    // Expand textarea dynamically in chat
    chatInput.addEventListener('input', () => {
        chatInput.style.height = 'auto';
        chatInput.style.height = (chatInput.scrollHeight - 4) + 'px';
    });

    btnClearChat.addEventListener('click', () => {
        chatMessages.innerHTML = '';
        chatHistory = [];
        appendChatMessage('system', 'Conversa limpa com sucesso.');
    });

    // 4. QUICK SEARCH MODAL SYSTEM
    function openSearchModal() {
        searchModal.classList.add('active');
        searchInput.value = '';
        searchResults.innerHTML = '<div class="search-helper-text">Comece a digitar para pesquisar documentos...</div>';
        setTimeout(() => searchInput.focus(), 50);
    }

    function closeSearchModal() {
        searchModal.classList.remove('active');
    }

    searchTriggerBtn.addEventListener('click', openSearchModal);
    btnCloseSearch.addEventListener('click', closeSearchModal);

    // Close on overlay click
    searchModal.addEventListener('click', (e) => {
        if (e.target === searchModal) closeSearchModal();
    });

    // Keyboard Shortcuts (Cmd+K or Ctrl+K opens, Escape closes)
    window.addEventListener('keydown', (e) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
            e.preventDefault();
            if (searchModal.classList.contains('active')) {
                closeSearchModal();
            } else {
                openSearchModal();
            }
        }
        if (e.key === 'Escape' && searchModal.classList.contains('active')) {
            closeSearchModal();
        }
    });

    // Debounced Search Query Input
    let searchDebounceTimeout = null;
    searchInput.addEventListener('input', () => {
        clearTimeout(searchDebounceTimeout);
        const query = searchInput.value.trim();
        if (!query) {
            searchResults.innerHTML = '<div class="search-helper-text">Comece a digitar para pesquisar documentos...</div>';
            return;
        }

        searchDebounceTimeout = setTimeout(() => {
            executeSearchQuery(query);
        }, 250);
    });

    async function executeSearchQuery(query) {
        searchResults.innerHTML = '<div class="search-helper-text">Pesquisando index...</div>';
        try {
            const res = await fetch('/api/wiki/query', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ query: query })
            });

            if (!res.ok) throw new Error('Pesquisa falhou');
            const results = await res.json();
            
            renderSearchResults(results);
        } catch (err) {
            console.error(err);
            searchResults.innerHTML = `<div class="search-helper-text" style="color: #ef4444;">Erro: ${err.message}</div>`;
        }
    }

    function renderSearchResults(results) {
        searchResults.innerHTML = '';
        if (!results || results.length === 0) {
            searchResults.innerHTML = '<div class="search-helper-text">Nenhum documento encontrado para essa busca.</div>';
            return;
        }

        results.forEach(res => {
            const card = document.createElement('div');
            card.className = 'result-card';
            
            const tagsHtml = res.tags ? res.tags.map(t => `<span class="tag-badge">${t}</span>`).join(' ') : '';

            card.innerHTML = `
                <div class="result-header">
                    <span class="result-title">${res.title}</span>
                    <span class="result-score">Score: ${res.score}</span>
                </div>
                <div class="result-snippet">${res.snippet}</div>
                <div class="result-meta">
                    <span>Caminho: ${res.path}</span>
                    <span>${tagsHtml}</span>
                </div>
            `;

            card.addEventListener('click', () => {
                closeSearchModal();
                selectDocument(res.path);
            });

            searchResults.appendChild(card);
        });
    }

    // 5. GRAPH VISUALIZATION SYSTEM (Canvas-based)
    let hoveredNode = null;
    const neighbors = new Set();
    const neighborLinks = new Set();

    function getGroupColor(group) {
        const colors = {
            '00-inbox': '#f59e0b',       // Amber/Yellow
            '01-global': '#06b6d4',      // Cyan
            '02-domains': '#a855f7',     // Purple
            '03-projects': '#3b82f6',    // Blue
            '04-systems': '#10b981',     // Emerald/Green
            '05-decisions': '#f43f5e',    // Rose/Red
            '06-agents': '#6366f1',       // Indigo
            '07-runbooks': '#14b8a6',     // Teal
            '08-prompts': '#8b5cf6',      // Violet
            '09-templates': '#6b7280',    // Gray
            '10-references': '#d97706',   // Brown/Orange
            'root': '#4b5563'             // Dark Gray
        };
        return colors[group] || '#6366f1'; // default Indigo
    }

    async function renderGraph() {
        graphContainer.innerHTML = '<div class="loading-state" style="padding:20px;">Carregando visualização de gráfico...</div>';
        
        try {
            const res = await fetch('/api/wiki/graph');
            if (!res.ok) throw new Error('Falha ao obter dados do gráfico');
            const graphData = await res.json();
            
            graphContainer.innerHTML = '';
            
            const nodes = graphData.nodes || [];
            const links = graphData.links || [];

            if (nodes.length === 0) {
                graphContainer.innerHTML = '<div class="empty-state" style="padding:20px;">Nenhum documento encontrado para gerar o gráfico.</div>';
                return;
            }

            // Calculate degrees (number of connections) to size the nodes
            const degrees = {};
            nodes.forEach(n => degrees[n.id] = 0);
            links.forEach(l => {
                degrees[l.source] = (degrees[l.source] || 0) + 1;
                degrees[l.target] = (degrees[l.target] || 0) + 1;
            });
            
            nodes.forEach(n => {
                const deg = degrees[n.id] || 0;
                n.val = 3 + Math.sqrt(deg) * 1.5; // log-like scaling for node size
            });

            // Initialize ForceGraph
            graphInstance = ForceGraph()(graphContainer)
                .width(graphContainer.clientWidth)
                .height(graphContainer.clientHeight)
                .graphData({ nodes, links })
                .backgroundColor('#0b0d10')
                .nodeId('id')
                .nodeVal('val')
                .linkSource('source')
                .linkTarget('target')
                // Node drawing
                .nodeCanvasObject((node, ctx, globalScale) => {
                    const label = node.title || node.id.split('/').pop().replace('.md', '');
                    const isHovered = (node === hoveredNode || neighbors.has(node.id));
                    const baseColor = getGroupColor(node.group);
                    const radius = node.val;
                    
                    // Node circle
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI, false);
                    
                    if (hoveredNode) {
                        if (node === hoveredNode) {
                            ctx.shadowColor = baseColor;
                            ctx.shadowBlur = 16;
                            ctx.fillStyle = '#ffffff';
                        } else if (neighbors.has(node.id)) {
                            ctx.shadowColor = baseColor;
                            ctx.shadowBlur = 10;
                            ctx.fillStyle = baseColor;
                        } else {
                            ctx.shadowBlur = 0;
                            ctx.fillStyle = 'rgba(100, 116, 139, 0.15)';
                        }
                    } else {
                        ctx.shadowBlur = 0;
                        ctx.fillStyle = baseColor;
                    }
                    ctx.fill();
                    ctx.shadowBlur = 0; // reset shadow

                    // Node Title text below
                    if (globalScale > 0.85 || isHovered) {
                        const fontSize = 11 / globalScale;
                        ctx.font = `${isHovered ? 'bold' : 'normal'} ${fontSize}px var(--font-sans)`;
                        ctx.textAlign = 'center';
                        ctx.textBaseline = 'middle';
                        ctx.fillStyle = isHovered ? '#ffffff' : 'rgba(243, 244, 246, 0.7)';
                        ctx.fillText(label, node.x, node.y + radius + fontSize * 0.9);
                    }
                })
                // Hover behavior
                .onNodeHover(node => {
                    neighbors.clear();
                    neighborLinks.clear();
                    hoveredNode = node || null;
                    if (node) {
                        links.forEach(link => {
                            if (link.source.id === node.id) {
                                neighbors.add(link.target.id);
                                neighborLinks.add(link);
                            } else if (link.target.id === node.id) {
                                neighbors.add(link.source.id);
                                neighborLinks.add(link);
                            }
                        });
                    }
                    // Refresh colors/thickness/particles
                    if (graphInstance && typeof graphInstance.refresh === 'function') {
                        graphInstance.refresh();
                    }
                })
                // Link styling
                .linkWidth(link => {
                    if (hoveredNode) {
                        return neighborLinks.has(link) ? 2.0 : 0.4;
                    }
                    return 1.0;
                })
                .linkColor(link => {
                    if (hoveredNode) {
                        return neighborLinks.has(link) ? 'rgba(99, 102, 241, 0.85)' : 'rgba(255, 255, 255, 0.015)';
                    }
                    return 'rgba(255, 255, 255, 0.09)';
                })
                .linkDirectionalParticles(link => {
                    // Show flowing particles along highlighted links on hover
                    if (hoveredNode && neighborLinks.has(link)) {
                        return 2;
                    }
                    return 0;
                })
                .linkDirectionalParticleWidth(2.5)
                .linkDirectionalParticleSpeed(0.006)
                // Click behavior: Navigate to file
                .onNodeClick(node => {
                    selectDocument(node.id);
                });

            // Adjust graph forces for Obsidian feel (nodes closer together)
            graphInstance.d3Force('charge').strength(-80);
            graphInstance.d3Force('link').distance(35);

            // Re-center on container resize
            window.addEventListener('resize', () => {
                if (activeMode === 'graph' && graphInstance) {
                    graphInstance.width(graphContainer.clientWidth);
                    graphInstance.height(graphContainer.clientHeight);
                }
            });

        } catch (err) {
            console.error(err);
            graphContainer.innerHTML = `<div class="error-state" style="padding:20px;">Erro ao inicializar o gráfico: ${err.message}</div>`;
        }
    }

    // Refresh tree button
    btnRefreshTree.addEventListener('click', () => {
        loadDirectoryTree();
    });
});
