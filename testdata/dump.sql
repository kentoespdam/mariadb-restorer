-- ============================================================================
-- MariaDB SQL Dump — Test File for mariadb-restorer
-- Covers all splitter/lexer features:
--   • Comments (--, #, /* */, /*!...*/)
--   • Single/double/backtick quoting with escapes
--   • Multi-char delimiter (stored procedures)
--   • SET sql_mode / SET NAMES
--   • Multi-line statements
--   • Various data types & encodings
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. Header metadata (mysqldump-style)
-- ---------------------------------------------------------------------------
-- Host: localhost    Database: test_restore
-- ------------------------------------------------------
-- Server version	10.6.18-MariaDB-log

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO,ANSI_QUOTES,NO_BACKSLASH_ESCAPES' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

-- ---------------------------------------------------------------------------
-- 2. Database & USE (single-line)
-- ---------------------------------------------------------------------------
CREATE DATABASE IF NOT EXISTS `test_restore` /*!40100 DEFAULT CHARACTER SET utf8mb4 */;
USE `test_restore`;

-- ---------------------------------------------------------------------------
-- 3. Tables — multi-line DDL with various column types
-- ---------------------------------------------------------------------------

--
-- Table structure for table `categories`
--
DROP TABLE IF EXISTS `categories`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `categories` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL,
  `description` text DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT current_timestamp(),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `products`
--
DROP TABLE IF EXISTS `products`;
CREATE TABLE `products` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `category_id` int(10) unsigned DEFAULT NULL,
  `sku` varchar(32) NOT NULL,
  `name` varchar(255) NOT NULL,
  `price` decimal(10,2) NOT NULL DEFAULT 0.00,
  `stock` int(11) NOT NULL DEFAULT 0,
  `metadata` longtext DEFAULT NULL CHECK (json_valid(`metadata`)),
  `created_at` timestamp NOT NULL DEFAULT current_timestamp(),
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE current_timestamp(),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sku` (`sku`),
  KEY `fk_category` (`category_id`),
  CONSTRAINT `fk_category` FOREIGN KEY (`category_id`) REFERENCES `categories` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB AUTO_INCREMENT=101 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ---------------------------------------------------------------------------
-- 4. INSERT data — including quotes, escapes, and multi-byte chars
-- ---------------------------------------------------------------------------

--
-- Dumping data for table `categories`
--
INSERT INTO `categories` VALUES
(1, 'Electronics', 'Devices, gadgets, and accessories', '2026-01-15 10:00:00'),
(2, 'Books', 'Physical and digital publications', '2026-01-15 10:00:00'),
(3, 'Home & Garden', 'Furniture, tools, and outdoor', '2026-01-15 10:00:00'),
(4, 'Clothing', 'Apparel with tricky sizes like S, M, L, XL', '2026-01-15 10:00:00');

--
-- Dumping data for table `products`
--
INSERT INTO `products` (`id`, `category_id`, `sku`, `name`, `price`, `stock`, `metadata`, `created_at`) VALUES
(1, 1, 'ELEC-001', 'Wireless Headphones with ''Active Noise Cancellation''', 149.99, 50, '{"color":"black","warranty":"1 year"}', '2026-02-01 08:30:00'),
(2, 1, 'ELEC-002', 'USB-C Hub 7-in-1 (4K@60Hz, 100W PD)', 39.99, 200, '{"ports":["HDMI","USB-A","USB-C","SD","3.5mm"],"power_delivery":"100W"}', '2026-02-01 08:30:00'),
(3, 1, 'ELEC-003', 'Portable SSD 2TB — max read 1050MB/s', 179.99, 75, '{"capacity":"2TB","interface":"USB 3.2 Gen 2","color":"space gray"}', '2026-02-01 08:30:00'),
(4, 2, 'BOOK-001', 'Learning SQL — 3rd Edition', 45.00, 30, '{"isbn":"978-1-098-12345-6","pages":620}', '2026-02-03 09:00:00'),
(5, 2, 'BOOK-002', 'MySQL Cookbook: Solutions for ''Everyday'' Development', 55.00, 20, '{"isbn":"978-1-492-09876-5","pages":840}', '2026-02-03 09:00:00'),
(6, 2, 'BOOK-003', 'Database Internals: ''A Deep Dive into \\\"How Distributed Systems Work\\\"''', 65.00, 15, '{"isbn":"978-1-492-04312-3","pages":370}', '2026-02-03 09:00:00'),
(7, 4, 'CLTH-001', 'T-Shirt ''Vintage'' Collection — Size L (Chest: 40\")', 24.99, 500, '{"sizes":["S","M","L","XL"],"material":"100% cotton","care":"Machine wash cold"}', '2026-02-05 14:00:00'),
(8, 4, 'CLTH-002', 'Denim Jacket — ''Classic'' Fit (Note: back-ordered)', 89.99, 0, '{"color":"indigo","material":"100% cotton denim","note":"Back-ordered until March"}', '2026-02-05 14:00:00'),
(9, 4, 'CLTH-003', 'Running Shoes — Size 42 (EU) / 8.5 (US)', 129.99, 35, '{"sizes":"38-46 EU","sole":"Rubber","upper":"Mesh"}', '2026-02-05 14:00:00'),
(10, 3, 'HOME-001', 'Spatula — Heat-resistant up to 500°F', 12.99, 1000, '{"material":"Silicone + Stainless Steel","dishwasher_safe":true}', '2026-02-10 11:00:00'),
(11, 3, 'HOME-002', 'Indoor Plant Pot — ø25cm (10\")', 19.99, 300, '{"material":"Ceramic","color":"terracotta","drainage":true}', '2026-02-10 11:00:00'),
(12, 3, 'HOME-003', 'Garden Hose 50ft — burst pressure 600 PSI', 34.99, 80, '{"length":"50ft (15m)","material":"Reinforced rubber","max_pressure":"600 PSI"}', '2026-02-10 11:00:00');

-- ---------------------------------------------------------------------------
-- 5. Hash comments (mysql style)
-- ---------------------------------------------------------------------------
# This is a hash comment — splitter handles it like -- comments.
# No special characters inside: just plain text.
INSERT INTO `products` (`id`, `category_id`, `sku`, `name`, `price`, `stock`, `metadata`, `created_at`) VALUES
(13, 1, 'ELEC-004', 'USB Cable — Type-C to C (2m/6.6ft)', 9.99, 500, '{}', '2026-02-15 16:00:00');

-- ---------------------------------------------------------------------------
-- 6. Complex quoting — nested quotes, backticks, double-quotes with ANSI_QUOTES
-- Note: SET NAMES and SET sql_mode above already enabled ANSI_QUOTES and
--       NO_BACKSLASH_ESCAPES, so double-quotes act as identifiers here.
-- ---------------------------------------------------------------------------

-- "products" is a valid identifier (quoted with double-quotes under ANSI_QUOTES).
INSERT INTO "products" ("id", "category_id", "sku", "name", "price", "stock", "metadata") VALUES
(14, 1, 'ELEC-005', '"Fast Charging" Brick — 65W GaN (2×USB-C, 1×USB-A)', 29.99, 120, '{"charge_standards":["PD 3.0","QC 4+","PPS"],"max_power":"65W"}');

-- ---------------------------------------------------------------------------
-- 7. Multi-line INSERT with one statement spanning many rows
-- ---------------------------------------------------------------------------
INSERT INTO `products` (`id`, `category_id`, `sku`, `name`, `price`, `stock`, `metadata`, `created_at`) VALUES
(15, 1, 'ELEC-006', 'Monitor 27\" 4K IPS — HDR400, USB-C 90W',
  449.99, 25,
  '{"resolution":"3840x2160","panel":"IPS","refresh_rate":"60Hz","hdr":"HDR400","usb_c_power_delivery":"90W"}',
  '2026-02-20 09:00:00'),
(16, 2, 'BOOK-004', 'Clean Code: A Handbook of Agile Software Craftsmanship',
  42.00, 60,
  '{"isbn":"978-0-132-35088-4","pages":464}',
  '2026-02-20 09:00:00'),
(17, 2, 'BOOK-005', 'Designing Data-Intensive Applications',
  48.00, 40,
  '{"isbn":"978-1-449-37332-0","pages":616}',
  '2026-02-20 09:00:00');

-- ---------------------------------------------------------------------------
-- 8. ALTER TABLE (single statement after inserts)
-- ---------------------------------------------------------------------------
ALTER TABLE `products` ADD INDEX `idx_name` (`name`(100));

-- ---------------------------------------------------------------------------
-- 9. UPDATE with subquery
-- ---------------------------------------------------------------------------
UPDATE `products` SET `stock` = `stock` + 10
WHERE `category_id` = (
  SELECT `id` FROM `categories` WHERE `name` = 'Electronics'
);

-- ---------------------------------------------------------------------------
-- 10. Stored procedure — multi-char delimiter
-- ---------------------------------------------------------------------------

DELIMITER $$

--
-- Procedure: get_products_by_category
-- Demonstrates multi-char delimiter, body with BEGIN…END, nested quoting
--
CREATE PROCEDURE `get_products_by_category`(IN cat_name VARCHAR(100))
BEGIN
  DECLARE cat_id INT DEFAULT 0;

  SELECT `id` INTO cat_id
  FROM `categories`
  WHERE `name` = cat_name
  LIMIT 1;

  IF cat_id > 0 THEN
    SELECT p.`id`, p.`sku`, p.`name`, p.`price`, p.`stock`
    FROM `products` p
    WHERE p.`category_id` = cat_id
    ORDER BY p.`name`;
  ELSE
    SELECT 'No such category: ' AS `message`, cat_name AS `category`;
  END IF;
END$$

--
-- Procedure: calculate_discount
-- With nested string containing semicolons inside the body
--
CREATE PROCEDURE `calculate_discount`(IN price DECIMAL(10,2), OUT discounted DECIMAL(10,2))
BEGIN
  -- Apply tiered discount
  IF price > 100 THEN
    SET discounted = price * 0.85;
  ELSEIF price > 50 THEN
    SET discounted = price * 0.90;
  ELSE
    SET discounted = price;
  END IF;
END$$

DELIMITER ;

-- ---------------------------------------------------------------------------
-- 11. Function with delimiter
-- ---------------------------------------------------------------------------

DELIMITER $$

CREATE FUNCTION `product_count`(cat_id INT) RETURNS INT
DETERMINISTIC
READS SQL DATA
BEGIN
  DECLARE cnt INT DEFAULT 0;
  SELECT COUNT(*) INTO cnt
  FROM `products`
  WHERE `category_id` = cat_id;
  RETURN cnt;
END$$

DELIMITER ;

-- ---------------------------------------------------------------------------
-- 12. Trigger
-- ---------------------------------------------------------------------------

DELIMITER $$

CREATE TRIGGER `products_before_update` BEFORE UPDATE ON `products`
FOR EACH ROW
BEGIN
  SET NEW.`updated_at` = CURRENT_TIMESTAMP;
END$$

DELIMITER ;

-- ---------------------------------------------------------------------------
-- 13. View — single statement
-- ---------------------------------------------------------------------------

DROP VIEW IF EXISTS `product_summary`;
CREATE VIEW `product_summary` AS
SELECT
  c.`name` AS `category`,
  COUNT(p.`id`) AS `product_count`,
  ROUND(AVG(p.`price`), 2) AS `avg_price`,
  SUM(p.`stock`) AS `total_stock`
FROM `categories` c
LEFT JOIN `products` p ON p.`category_id` = c.`id`
GROUP BY c.`id`, c.`name`
ORDER BY c.`name`;

-- ---------------------------------------------------------------------------
-- 14. Event — scheduled event
-- ---------------------------------------------------------------------------

DELIMITER $$

CREATE EVENT `cleanup_old_products`
ON SCHEDULE EVERY 1 DAY
STARTS '2026-07-16 02:00:00'
DO
BEGIN
  DELETE FROM `products`
  WHERE `created_at` < DATE_SUB(NOW(), INTERVAL 5 YEAR)
  AND `stock` = 0;
END$$

DELIMITER ;

-- ---------------------------------------------------------------------------
-- 15. Grant (in executable comment so non-root users can skip)
-- ---------------------------------------------------------------------------
/*!40000 GRANT SELECT ON `test_restore`.* TO 'reader'@'%' */;

-- ---------------------------------------------------------------------------
-- 16. Final INSERT with special characters in data
-- ---------------------------------------------------------------------------
INSERT INTO `products` (`id`, `category_id`, `sku`, `name`, `price`, `stock`, `metadata`, `created_at`) VALUES
(18, 3, 'HOME-004', 'String Lights — 20m (65ft) with 200 LEDs ''Warm White''', 22.99, 250, '{}', '2026-03-01 18:30:00');

-- ---------------------------------------------------------------------------
-- 17. Footer (mysqldump restore footer)
-- ---------------------------------------------------------------------------
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;
/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2026-07-15 12:00:00
