package handlers

import (
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// CreateInventoryTables создаёт таблицы для складского учёта
func CreateInventoryTables(c *gin.Context) {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS inventory_products (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255) NOT NULL,
            description TEXT,
            sku VARCHAR(100),
            barcode VARCHAR(100),
            price DECIMAL(15,2) DEFAULT 0,
            cost DECIMAL(15,2) DEFAULT 0,
            quantity INT DEFAULT 0,
            reserved_quantity INT DEFAULT 0,
            min_stock INT DEFAULT 0,
            max_stock INT DEFAULT 0,
            location VARCHAR(100),
            unit VARCHAR(20) DEFAULT 'шт',
            is_active BOOLEAN DEFAULT TRUE,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        );`,
        
        `CREATE INDEX IF NOT EXISTS idx_products_sku ON inventory_products(sku);`,
        `CREATE INDEX IF NOT EXISTS idx_products_barcode ON inventory_products(barcode);`,
        
        `CREATE TABLE IF NOT EXISTS inventory_movements (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            product_id UUID REFERENCES inventory_products(id),
            movement_type VARCHAR(50),
            quantity INT NOT NULL,
            before_quantity INT,
            after_quantity INT,
            reference_id UUID,
            reference_type VARCHAR(50),
            comment TEXT,
            created_by UUID,
            created_at TIMESTAMP DEFAULT NOW()
        );`,
        
        `CREATE INDEX IF NOT EXISTS idx_movements_product ON inventory_movements(product_id);`,
    }
    
    for _, q := range queries {
        if _, err := database.Pool.Exec(c.Request.Context(), q); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "query": q})
            return
        }
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Таблицы созданы"})
}