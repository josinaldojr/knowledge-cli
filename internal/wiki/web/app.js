// ==========================================================================
// WIKI PREMIUM INTERACTIVE JAVASCRIPT
// ==========================================================================

document.addEventListener('DOMContentLoaded', () => {
    // STATE MANAGEMENT
    let activePath = null;
    let chatHistory = [];
    const expandedFolders = new Set(['01-global', '04-systems', '05-decisions']); // expand by default

    // DOM ELEMENTS
    const vaultTree = document.getElementById('vault-tree');
    const docViewer = document.getElementById('doc-viewer');
    const chatMessages = document.getElementById('chat-messages');
    const chatForm = document.getElementById('chat-form');
    const chatInput = document.getElementById('chat-input');
    const btnClearChat = document.getElementById('btn-clear-chat');
    const btnRefreshTree = document.getElementById('btn-refresh-tree');

    // SEARCH MODAL DOM
    const searchTriggerBtn = document.getElementById('search-trigger-btn');
    const searchModal = document.getElementById('search-modal');
    const searchInput = document.getElementById('search-input');
    const searchResults = document.getElementById('search-results');
    const btnCloseSearch = document.getElementById('btn-close-search');

    // INITIALIZATION
    loadDirectoryTree();

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
        activePath = path;
        
        // Update active class in sidebar tree
        document.querySelectorAll('.tree-file').forEach(el => {
            el.classList.remove('active');
        });
        const currentLinks = document.querySelectorAll(`a[href="#${path}"]`);
        currentLinks.forEach(link => link.classList.add('active'));

        docViewer.innerHTML = '<div class="loading-state">Carregando documento...</div>';

        try {
            const res = await fetch(`/api/wiki/doc?path=${encodeURIComponent(path)}`);
            if (!res.ok) throw new Error('Não foi possível ler o arquivo');
            const data = await res.json();
            
            renderMarkdown(data.content, path);
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

        // Parse markdown content using Marked.js
        html += marked.parse(markdown);

        docViewer.innerHTML = html;
        docViewer.scrollTop = 0;

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

    // Refresh tree button
    btnRefreshTree.addEventListener('click', () => {
        loadDirectoryTree();
    });
});
