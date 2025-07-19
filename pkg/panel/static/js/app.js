/**
 * Main application controller for Leads Core Admin Panel
 */

class AdminPanel {
    constructor() {
        // Initialize after DOM is loaded
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => this.init());
        } else {
            this.init();
        }
    }

    /**
     * Initialize the application
     */
    async init() {
        this.bindEvents();
        await this.checkDemoModeAvailability();
        window.UI.hideLoading();
        
        // Check if already authenticated
        if (window.AuthManager.isAuthenticated()) {
            await window.Dashboard.showDashboard();
        } else {
            window.Dashboard.showLogin();
        }
    }

    /**
     * Check if demo mode is available from server
     */
    async checkDemoModeAvailability() {
        try {
            // Try to make a request without auth to see if demo mode is enabled
            const response = await fetch('/api/v1/user', {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json'
                }
            });

            if (response.ok) {
                // Demo mode is enabled - show demo section
                const demoSection = document.getElementById('demo-section');
                if (demoSection) {
                    demoSection.style.display = 'block';
                }
            }
        } catch (error) {
            // Demo mode not available, keep demo section hidden
            console.log('Demo mode not available');
        }
    }

    /**
     * Bind event listeners
     */
    bindEvents() {
        // Login widget
        const loginWidget = document.getElementById('login-widget');
        if (loginWidget) {
            loginWidget.addEventListener('submit', (e) => this.handleLogin(e));
        }

        // Demo login button
        const demoLoginBtn = document.getElementById('demo-login-btn');
        if (demoLoginBtn) {
            demoLoginBtn.addEventListener('click', () => this.handleDemoLogin());
        }

        // Logout button
        const logoutBtn = document.getElementById('logout-btn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', () => this.handleLogout());
        }

        // Refresh button
        const refreshBtn = document.getElementById('refresh-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => window.Dashboard.refreshData());
        }

        // Create Widget button
        const createWidgetBtn = document.getElementById('create-widget-btn');
        if (createWidgetBtn) {
            createWidgetBtn.addEventListener('click', () => window.WidgetsManager.showCreateWidget());
        }

        // Search and filters
        this.setupSearchAndFilters();

        // Pagination
        this.setupPagination();
    }

    /**
     * Setup search and filter event listeners
     */
    setupSearchAndFilters() {
        // Search input
        const searchInput = document.getElementById('search-input');
        if (searchInput) {
            const debouncedSearch = window.UI.debounce((value) => {
                window.Dashboard.handleSearch(value);
            }, 500);
            
            searchInput.addEventListener('input', (e) => {
                debouncedSearch(e.target.value);
            });
        }

        // Status filter
        const filterStatus = document.getElementById('filter-status');
        if (filterStatus) {
            filterStatus.addEventListener('change', (e) => {
                window.Dashboard.handleFilter('status', e.target.value);
            });
        }

        // Type filter
        const filterType = document.getElementById('filter-type');
        if (filterType) {
            filterType.addEventListener('change', (e) => {
                window.Dashboard.handleFilter('type', e.target.value);
            });
        }
    }

    /**
     * Setup pagination event listeners
     */
    setupPagination() {
        const prevBtn = document.getElementById('prev-page');
        const nextBtn = document.getElementById('next-page');
        
        if (prevBtn) {
            prevBtn.addEventListener('click', () => window.Dashboard.previousPage());
        }
        
        if (nextBtn) {
            nextBtn.addEventListener('click', () => window.Dashboard.nextPage());
        }
    }

    /**
     * Handle login widget submission
     */
    async handleLogin(event) {
        event.preventDefault();
        
        const widget = event.target;
        const submitBtn = widget.querySelector('button[type="submit"]');
        const secretInput = widget.querySelector('#secret');
        const userIdInput = widget.querySelector('#userId');
        
        const secret = secretInput.value.trim();
        const userId = userIdInput.value.trim();
        
        if (!secret || !userId) {
            window.UI.showError('Please fill in all fields');
            return;
        }
        
        try {
            window.UI.hideError();
            window.UI.setButtonLoading(submitBtn, true);
            
            // Attempt login
            const result = await window.AuthManager.login(secret, userId);
            
            if (result.success) {
                // Set API token
                window.APIClient.setToken(result.token);
                
                // Test authentication
                await window.APIClient.testAuth();
                
                window.UI.showToast('Successfully authenticated!', 'success');
                await window.Dashboard.showDashboard();
            } else {
                window.UI.showError(result.error || 'Authentication failed');
            }
            
        } catch (error) {
            console.error('Login error:', error);
            window.UI.showError('Authentication failed: ' + error.message);
        } finally {
            window.UI.setButtonLoading(submitBtn, false);
        }
    }

    /**
     * Handle demo login
     */
    async handleDemoLogin() {
        try {
            // Enable demo mode
            window.AuthManager.enableDemoMode();
            
            // Test demo authentication
            await window.APIClient.testAuth();
            
            window.UI.showToast('Demo mode activated!', 'success');
            await window.Dashboard.showDashboard();
            
        } catch (error) {
            console.error('Demo login error:', error);
            window.UI.showError('Demo mode failed: ' + error.message);
            window.AuthManager.disableDemoMode();
        }
    }

    /**
     * Handle logout
     */
    handleLogout() {
        window.AuthManager.logout();
        window.APIClient.clearAuth();
        window.Dashboard.clearAutoRefresh();
        window.UI.closeAllModals();
        window.UI.showToast('Logged out successfully', 'info');
        window.Dashboard.showLogin();
    }
}

// Initialize the application
const adminPanel = new AdminPanel();

// Make it globally available for event handlers
window.AdminPanel = adminPanel;
