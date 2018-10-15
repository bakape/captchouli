extern "C" {
#include "thumbnail.h"
}
#include <cstring>
#include <functional>
#include <iostream>
#include <opencv2/opencv.hpp>
#include <stdexcept>

// Size of thumbnail dimension. thumbnail is always a square.
static const int thumb_dim = 150;

static const char* _thumbnail(
    cv::CascadeClassifier* c, const char* path, Buffer* thumb)
{
    cv::Mat colour = cv::imread(path, cv::IMREAD_COLOR);
    cv::Mat grayscale, equalized;
    cv::cvtColor(colour, grayscale, cv::COLOR_BGR2GRAY);
    cv::equalizeHist(grayscale, equalized);

    std::vector<cv::Rect> faces;
    c->detectMultiScale(equalized, faces, 1.1, 5, 0, cv::Size(50, 50));
    if (!faces.size()) {
        return "no faces detected";
    }

    cv::Rect face;
    if (faces.size() == 1) {
        face = faces.front();
    } else {
        // Find biggest match
        uint64_t max_size = 0;
        for (auto& f : faces) {
            uint64_t s = (uint64_t)f.width + (uint64_t)f.height;
            if (s > max_size) {
                face = f;
                max_size = s;
            }
        }
    }

    // Increase matched size, if image bellow 150x150.
    // face should always be a square.
    if (face.width < thumb_dim && face.height == face.width) {
        // Perform bounds checks and find the largest equal increase size in all
        // directions
        int diff = (150 - face.width) / 2;
        if (face.x - diff < 0) {
            diff = face.x;
        }
        if (face.y - diff < 0) {
            diff = face.y;
        }
        if (face.x + face.width + diff > colour.cols) {
            diff = colour.cols - (face.x + face.width);
        }
        if (face.y + face.height + diff > colour.rows) {
            diff = colour.rows - (face.y + face.height);
        }
        face.x -= diff;
        face.y -= diff;
        face.width += diff;
        face.height += diff;
    }

    cv::Mat resized;
    cv::resize(cv::Mat(colour, face), resized, cv::Size(thumb_dim, thumb_dim),
        0, 0, CV_INTER_LINEAR);
    std::vector<unsigned char> out;
    static const std::vector<int> params = { CV_IMWRITE_JPEG_QUALITY, 90 };
    if (!cv::imencode(".jpg", resized, out, params)) {
        throw std::domain_error("could not encode result");
    }
    thumb->data = memcpy(malloc(out.size()), out.data(), out.size());
    thumb->size = out.size();
    return 0;
}

static char* malloc_string(const char* s)
{
    return strcpy((char*)malloc(strlen(s) + 1), s);
}

static char* catch_errors(std::function<const char*()> fn)
{
    try {
        auto err = fn();
        if (err) {
            return malloc_string(err);
        }
        return nullptr;
    } catch (const std::exception& ex) {
        auto err = ex.what();
        return malloc_string(err);
    }
}

extern "C" void* load_classifier(const char* path)
{
    auto c = new cv::CascadeClassifier();
    if (!c->load(path)) {
        delete c;
        return nullptr;
    }
    return c;
}

extern "C" void unload_classifier(void* c)
{
    delete static_cast<cv::CascadeClassifier*>(c);
}

extern "C" char* thumbnail(void* classifier, const char* path, Buffer* thumb)
{
    return catch_errors([=]() {
        return _thumbnail(
            static_cast<cv::CascadeClassifier*>(classifier), path, thumb);
    });
}