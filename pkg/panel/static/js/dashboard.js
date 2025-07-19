/**
 * Dashboard management module for Leads Core Admin Panel
 */

class DashboardManager {
    constructor() {
        this.currentPage = 1;
        this.perPage = 20;
        this.totalWidgets = 0;
        this.currentFilters = {
            search: '',
            status: '',
            type: ''
        };
        this.refreshInterval = null;
    }

    /**
     * Show login section
     */
    showLogin() {
        const loginSection = document.getElementById('login-section');
        const dashboardSection = document.getElementById('dashboard-section');
        
        if (loginSection) loginSection.style.display = 'flex';
        if (dashboardSection) dashboardSection.style.display = 'none';
        
        window.UI.updatePageTitle('Login');
    }

    /**
     * Show dashboard section
     */
    async showDashboard() {
        try {
            window.UI.showLoading();
            
            const loginSection = document.getElementById('login-section');
            const dashboardSection = document.getElementById('dashboard-section');
            
            if (loginSection) loginSection.style.display = 'none';
            if (dashboardSection) dashboardSection.style.display = 'block';
            
            // Load and update user info
            await this.updateUserInfo();
            
            // Load dashboard data
            await this.loadData();
            
            // Setup auto-refresh
            this.setupAutoRefresh();
            
            window.UI.updatePageTitle('Dashboard');
            
        } catch (error) {
            console.error('Error showing dashboard:', error);
            window.UI.showToast('Error loading dashboard: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Update user information in header
     */
    async updateUserInfo() {
        try {
            const user = await window.APIClient.getCurrentUser();
            
            const userDisplay = document.getElementById('user-display');
            const userPlan = document.getElementById('user-plan');
            
            if (userDisplay && user) {
                const displayName = user.username || user.id;
                userDisplay.textContent = `${displayName}`;
            }
            
            if (userPlan && user) {
                userPlan.textContent = user.plan || 'free';
                userPlan.className = `user-plan ${user.plan || 'free'}`;
            }
            
        } catch (error) {
            console.error('Error loading user info:', error);
            
            // Fallback to stored user data
            const user = window.AuthManager.getUser();
            const userDisplay = document.getElementById('user-display');
            const userPlan = document.getElementById('user-plan');
            
            if (userDisplay && user) {
                const displayName = user.username || user.id;
                userDisplay.textContent = `${displayName}`;
            }
            
            if (userPlan && user) {
                userPlan.textContent = user.plan || 'free';
                userPlan.className = `user-plan ${user.plan || 'free'}`;
            }
        }
    }

    /**
     * Load all dashboard data
     */
    async loadData() {
        try {
            // Load summary data and widgets in parallel
            await Promise.all([
                this.loadSummary(),
                this.loadWidgets()
            ]);
        } catch (error) {
            console.error('Error loading data:', error);
            window.UI.showToast('Error loading data: ' + error.message, 'error');
        }
    }

    /**
     * Load summary statistics
     */
    async loadSummary() {
        try {
            const summary = await window.APIClient.getWidgetsSummary();
            
            // Update summary cards
            this.updateSummaryCard('total-widgets', summary.total_widgets || 0);
            this.updateSummaryCard('active-widgets', summary.active_widgets || 0);
            this.updateSummaryCard('total-views', summary.total_views || 0);
            this.updateSummaryCard('total-submissions', summary.total_submissions || 0);
            
        } catch (error) {
            console.error('Error loading summary:', error);
            // Set default values on error
            this.updateSummaryCard('total-widgets', 0);
            this.updateSummaryCard('active-widgets', 0);
            this.updateSummaryCard('total-views', 0);
            this.updateSummaryCard('total-submissions', 0);
        }
    }

    /**
     * Update summary card value
     */
    updateSummaryCard(elementId, value) {
        const element = document.getElementById(elementId);
        if (element) {
            element.textContent = window.UI.formatNumber(value);
        }
    }

    /**
     * Load widgets list
     */
    async loadWidgets() {
        try {
            const options = {
                page: this.currentPage,
                per_page: this.perPage
            };
            
            // Add search filter
            if (this.currentFilters.search && this.currentFilters.search.trim()) {
                options.search = this.currentFilters.search.trim();
            }
            
            // Add type filter
            if (this.currentFilters.type && this.currentFilters.type !== '') {
                options.type = this.currentFilters.type;
            }
            
            // Add visibility filter (convert status to isVisible)
            if (this.currentFilters.status && this.currentFilters.status !== '') {
                if (this.currentFilters.status === 'enabled') {
                    options.isVisible = true;
                } else if (this.currentFilters.status === 'disabled') {
                    options.isVisible = false;
                }
            }
            
            console.log('Loading widgets with options:', options); // Debug log
            
            const response = await window.APIClient.getWidgets(options);
            this.totalWidgets = response.meta?.total || 0;
            
            this.renderWidgetsTable(response.widgets || []);
            this.updatePagination();
            this.populateFilterOptions(response.meta?.type_stats || []);
            
        } catch (error) {
            console.error('Error loading widgets:', error);
            this.renderWidgetsTable([]);
            this.updatePagination();
        }
    }

    /**
     * Render widgets table
     */
    renderWidgetsTable(widgets) {
        const tbody = document.getElementById('widgets-tbody');
        if (!tbody) return;
        
        if (widgets.length === 0) {
            tbody.innerHTML = `
                <tr class="no-data-row">
                    <td colspan="8">
                        <div class="no-data">
                            <div class="no-data-icon">📝</div>
                            <div class="no-data-text">No widgets found</div>
                            <div class="no-data-subtext">
                                ${this.currentFilters.search || this.currentFilters.status || this.currentFilters.type 
                                    ? 'Try adjusting your search filters' 
                                    : 'Create your first widget to get started'}
                            </div>
                        </div>
                    </td>
                </tr>
            `;
            return;
        }
        
        tbody.innerHTML = widgets.map(widget => `
            <tr class="widget-row" data-widget-id="${widget.id}">
                <td class="widget-id">${widget.id}</td>
                <td class="widget-name">
                    <div class="widget-name-container">
                        <span class="name">${window.UI.escapeHtml(widget.name || 'Untitled')}</span>
                        ${widget.description ? `<span class="description">${window.UI.escapeHtml(widget.description)}</span>` : ''}
                    </div>
                </td>
                <td class="widget-type">
                    <span class="type-badge type-${(widget.type || 'other').replace(/([A-Z])/g, '-$1').toLowerCase()}">
                        ${this.formatWidgetTypeName(widget.type) || 'Other'}
                    </span>
                </td>
                <td class="widget-status">
                    <span class="status-badge ${widget.isVisible ? 'enabled' : 'disabled'}">
                        ${widget.isVisible ? '✅ Active' : '❌ Disabled'}
                    </span>
                </td>
                <td class="widget-created">${window.UI.formatDate(widget.createdAt || widget.created_at)}</td>
                <td class="widget-views">${window.UI.formatNumber(widget.stats?.views || 0)}</td>
                <td class="widget-submissions">${window.UI.formatNumber(widget.stats?.submits || 0)}</td>
                <td class="widget-actions">
                    <div class="table-actions">
                        <button class="btn-icon" onclick="window.UI.showWidgetDetails('${widget.id}')" title="View Details">
                            👁️
                        </button>
                        <button class="btn-icon" onclick="window.WidgetsManager.editWidget('${widget.id}')" title="Edit Widget">
                            ✏️
                        </button>
                        <button class="btn-icon" onclick="window.WidgetsManager.showExportModal('${widget.id}')" title="Export Submissions">
                            📥
                        </button>
                        <button class="btn-icon" onclick="window.WidgetsManager.toggleWidgetStatus('${widget.id}', ${!widget.isVisible})" title="${widget.isVisible ? 'Disable' : 'Enable'}">
                            ${widget.isVisible ? '⏸️' : '▶️'}
                        </button>
                        <button class="btn-icon btn-danger" onclick="window.WidgetsManager.deleteWidget('${widget.id}', '${window.UI.escapeHtml(widget.name || 'Untitled')}')" title="Delete Widget">
                            🗑️
                        </button>
                    </div>
                </td>
            </tr>
        `).join('');
    }

    /**
     * Update pagination controls
     */
    updatePagination() {
        const totalPages = Math.ceil(this.totalWidgets / this.perPage) || 1;
        
        // Update pagination info
        const paginationInfo = document.getElementById('pagination-info');
        const pageInfo = document.getElementById('page-info');
        const prevBtn = document.getElementById('prev-page');
        const nextBtn = document.getElementById('next-page');
        const paginationContainer = document.querySelector('.pagination-container');
        
        if (this.totalWidgets === 0) {
            if (paginationContainer) {
                paginationContainer.style.display = 'none';
            }
            return;
        }
        
        if (paginationContainer) {
            paginationContainer.style.display = 'flex';
        }
        
        const startItem = ((this.currentPage - 1) * this.perPage) + 1;
        const endItem = Math.min(this.currentPage * this.perPage, this.totalWidgets);
        
        if (paginationInfo) {
            paginationInfo.textContent = `Showing ${startItem}-${endItem} of ${this.totalWidgets} widgets`;
        }
        
        if (pageInfo) {
            pageInfo.textContent = `Page ${this.currentPage} of ${totalPages}`;
        }
        
        if (prevBtn) {
            prevBtn.disabled = this.currentPage <= 1;
        }
        
        if (nextBtn) {
            nextBtn.disabled = this.currentPage >= totalPages;
        }
    }

    /**
     * Populate filter options
     */
    populateFilterOptions(typeStats) {
        const typeFilter = document.getElementById('filter-type');
        if (!typeFilter) return;
        
        // Define all available widget types - synchronized with backend models
        const availableTypes = [
            { value: 'lead-form', label: 'Lead Form' },
            { value: 'banner', label: 'Banner' },
            { value: 'action', label: 'Action' },
            { value: 'social-proof', label: 'Social Proof' },
            { value: 'live-interest', label: 'Live Interest' },
            { value: 'widget-tab', label: 'Widget Tab' },
            { value: 'sticky-bar', label: 'Sticky Bar' },
            { value: 'quiz', label: 'Quiz' },
            { value: 'wheelOfFortune', label: 'Wheel of Fortune' },
            { value: 'survey', label: 'Survey' },
            { value: 'coupon', label: 'Coupon' }
        ];
        
        // Create a map of type counts from statistics
        const typeCountMap = {};
        typeStats.forEach(stat => {
            typeCountMap[stat.type] = stat.count;
        });
        
        // Clear current options (except "All Types")
        typeFilter.innerHTML = '<option value="">All Types</option>';
        
        // Add type options (show all types with counts from statistics)
        availableTypes.forEach(typeInfo => {
            const option = document.createElement('option');
            option.value = typeInfo.value;
            const count = typeCountMap[typeInfo.value] || 0;
            
            if (count > 0) {
                option.textContent = `${typeInfo.label} (${count})`;
            } else {
                option.textContent = `${typeInfo.label} (0)`;
                option.style.color = '#999';
            }
            
            typeFilter.appendChild(option);
        });
        
        // Restore current selection if it exists
        if (this.currentFilters.type) {
            typeFilter.value = this.currentFilters.type;
        }
    }

    /**
     * Handle search input
     */
    handleSearch(query) {
        this.currentFilters.search = query;
        this.currentPage = 1;
        this.loadWidgets();
    }

    /**
     * Handle filter change
     */
    handleFilter(filterType, value) {
        this.currentFilters[filterType] = value;
        this.currentPage = 1;
        this.loadWidgets();
    }

    /**
     * Go to previous page
     */
    previousPage() {
        if (this.currentPage > 1) {
            this.currentPage--;
            this.loadWidgets();
        }
    }

    /**
     * Go to next page
     */
    nextPage() {
        const totalPages = Math.ceil(this.totalWidgets / this.perPage);
        if (this.currentPage < totalPages) {
            this.currentPage++;
            this.loadWidgets();
        }
    }

    /**
     * Refresh all data
     */
    async refreshData() {
        try {
            window.UI.showLoading();
            await Promise.all([
                this.updateUserInfo(),
                this.loadData()
            ]);
            window.UI.showToast('Data refreshed successfully', 'success');
        } catch (error) {
            console.error('Error refreshing data:', error);
            window.UI.showToast('Error refreshing data: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Setup auto-refresh interval
     */
    setupAutoRefresh() {
        // Clear existing interval
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        
        // Set up new interval (refresh every 30 seconds)
        this.refreshInterval = setInterval(() => {
            if (window.AuthManager.isAuthenticated()) {
                this.loadData();
            } else {
                this.clearAutoRefresh();
            }
        }, 30000);
    }

    /**
     * Clear auto-refresh interval
     */
    clearAutoRefresh() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
            this.refreshInterval = null;
        }
    }

    /**
     * Reset filters
     */
    resetFilters() {
        this.currentFilters = {
            search: '',
            status: '',
            type: ''
        };
        this.currentPage = 1;
        
        // Reset UI elements
        const searchInput = document.getElementById('search-input');
        const statusFilter = document.getElementById('filter-status');
        const typeFilter = document.getElementById('filter-type');
        
        if (searchInput) searchInput.value = '';
        if (statusFilter) statusFilter.value = '';
        if (typeFilter) typeFilter.value = '';
        
        this.loadWidgets();
    }

    /**
     * Format widget type name for display
     */
    formatWidgetTypeName(type) {
        const typeMap = {
            'lead-form': 'Lead Form',
            'banner': 'Banner',
            'action': 'Action',
            'social-proof': 'Social Proof',
            'live-interest': 'Live Interest',
            'widget-tab': 'Widget Tab',
            'sticky-bar': 'Sticky Bar',
            'quiz': 'Quiz',
            'wheelOfFortune': 'Wheel of Fortune'
        };
        
        return typeMap[type] || type;
    }
}

// Create global instance
console.log('Creating Dashboard instance...');
try {
    window.Dashboard = new DashboardManager();
    console.log('Dashboard instance created successfully:', window.Dashboard);
} catch (error) {
    console.error('Error creating Dashboard instance:', error);
}
